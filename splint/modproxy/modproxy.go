// Package modproxy asks the Go module proxy what a dependency is.
//
// It answers three things a document cannot: how big the module is, when the
// version in use was published, and what the latest one is. All three come
// from the proxy protocol without downloading anything: the size is the
// Content-Length of a HEAD on the zip, and the rest is one small JSON document
// each.
//
// Nothing here is required for a report. A machine with no network answers
// nothing and says so, and the caller reports what the document told it.
package modproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultProxy is where a module is asked about when GOPROXY says nothing.
const DefaultProxy = "https://proxy.golang.org"

// Info is what the proxy knows about one module version.
type Info struct {
	// Path and Version are the module asked about.
	Path    string `json:"Path"`
	Version string `json:"Version"`

	// Size is the module zip in bytes, which is what a consumer downloads. It
	// is the compressed size: the proxy reports it without unpacking anything,
	// and unpacking every dependency to count the bytes inside costs more than
	// the answer is worth.
	Size int64 `json:"Size,omitempty"`

	// Published is when the version in use was tagged, and Latest is the
	// newest version the proxy knows of.
	Published time.Time `json:"Published,omitzero"`
	Latest    string    `json:"Latest,omitempty"`

	// LatestPublished is when that newest version was tagged, which is what
	// says whether a dependency is behind or simply finished.
	LatestPublished time.Time `json:"LatestPublished,omitzero"`

	// Err is why the proxy could not answer, and is empty when it did.
	Err string `json:"Err,omitempty"`
}

// Behind reports a module pinned to something older than the latest.
func (i Info) Behind() bool {
	return i.Latest != "" && i.Version != "" && i.Latest != i.Version
}

// Age is how long ago the version in use was published.
func (i Info) Age(now time.Time) time.Duration {
	if i.Published.IsZero() {
		return 0
	}
	return now.Sub(i.Published)
}

// Client asks a proxy about modules.
//
// It is safe for concurrent use and remembers what it was told: a module
// version is immutable, so the same question has the same answer for the life
// of the process.
type Client struct {
	// Proxy is the base URL, and HTTP is what talks to it.
	Proxy string
	HTTP  *http.Client

	// Private are the module path prefixes not to ask about, which is what
	// GOPRIVATE and GONOPROXY name. A private module is nobody's business but
	// the person building it.
	Private []string

	mu    sync.Mutex
	known map[string]Info
}

// New returns a client reading GOPROXY, GOPRIVATE and GONOPROXY the way the
// toolchain reads them.
//
// A GOPROXY listing several proxies is read down to the first real URL: the
// fallbacks are what the toolchain does when a download fails, and this asks
// one question that either answers or does not.
func New() *Client {
	return &Client{
		Proxy:   proxyFromEnv(),
		HTTP:    &http.Client{Timeout: 10 * time.Second},
		Private: privateFromEnv(),
		known:   map[string]Info{},
	}
}

// Lookup asks about one module version.
func (c *Client) Lookup(ctx context.Context, path, version string) Info {
	key := path + "@" + version

	c.mu.Lock()
	if info, seen := c.known[key]; seen {
		c.mu.Unlock()
		return info
	}
	c.mu.Unlock()

	info := c.ask(ctx, path, version)

	c.mu.Lock()
	c.known[key] = info
	c.mu.Unlock()

	return info
}

// LookupAll asks about many modules at once, which is what a report needs: a
// tree of thirty dependencies asked about one at a time is thirty round trips
// end to end.
func (c *Client) LookupAll(ctx context.Context, modules map[string]string) map[string]Info {
	const workers = 8

	type question struct{ path, version string }

	queue := make(chan question)
	out := make(chan Info)

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for q := range queue {
				out <- c.Lookup(ctx, q.path, q.version)
			}
		}()
	}

	go func() {
		for path, version := range modules {
			queue <- question{path, version}
		}
		close(queue)
		wg.Wait()
		close(out)
	}()

	answers := make(map[string]Info, len(modules))
	for info := range out {
		answers[info.Path] = info
	}
	return answers
}

// ask is one module's three questions.
func (c *Client) ask(ctx context.Context, path, version string) Info {
	info := Info{Path: path, Version: version}

	if c.Proxy == "" || c.private(path) {
		info.Err = "not asked: the module is private or no proxy is configured"
		return info
	}

	escaped, err := escapePath(path)
	if err != nil {
		info.Err = err.Error()
		return info
	}

	if size, err := c.size(ctx, escaped, version); err != nil {
		info.Err = err.Error()
	} else {
		info.Size = size
	}

	if published, err := c.published(ctx, escaped, version); err == nil {
		info.Published = published
	}

	if latest, published, err := c.latest(ctx, escaped); err == nil {
		info.Latest = latest
		info.LatestPublished = published
	}

	return info
}

// size is the module zip in bytes, read from the header of a HEAD request. The
// zip is never fetched.
func (c *Client) size(ctx context.Context, escaped, version string) (int64, error) {
	response, err := c.do(ctx, http.MethodHead, fmt.Sprintf("%s/%s/@v/%s.zip", c.Proxy, escaped, version))
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()

	length := response.Header.Get("Content-Length")
	if length == "" {
		return 0, fmt.Errorf("the proxy reported no size")
	}
	return strconv.ParseInt(length, 10, 64)
}

// published is when a version was tagged.
func (c *Client) published(ctx context.Context, escaped, version string) (time.Time, error) {
	var out struct{ Time time.Time }
	err := c.json(ctx, fmt.Sprintf("%s/%s/@v/%s.info", c.Proxy, escaped, version), &out)
	return out.Time, err
}

// latest is the newest version the proxy knows of, and when it was tagged.
func (c *Client) latest(ctx context.Context, escaped string) (string, time.Time, error) {
	var out struct {
		Version string
		Time    time.Time
	}
	err := c.json(ctx, fmt.Sprintf("%s/%s/@latest", c.Proxy, escaped), &out)
	return out.Version, out.Time, err
}

// json reads one JSON document from the proxy.
func (c *Client) json(ctx context.Context, url string, into any) error {
	response, err := c.do(ctx, http.MethodGet, url)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, into)
}

// do makes one request and refuses anything that is not an answer.
func (c *Client) do(ctx context.Context, method, url string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}

	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("the proxy answered %s", response.Status)
	}
	return response, nil
}

// private reports a module the proxy is not to be asked about.
func (c *Client) private(path string) bool {
	for _, prefix := range c.Private {
		if prefix == "" {
			continue
		}
		if path == prefix || strings.HasPrefix(path, strings.TrimSuffix(prefix, "/*")) {
			return true
		}
	}
	return false
}

// escapePath spells a module path the way the proxy protocol does, with every
// upper case letter written as a bang and its lower case: the protocol is
// served off case insensitive filesystems, and two modules differing only in
// case would otherwise be one file.
func escapePath(path string) (string, error) {
	var out strings.Builder

	for _, r := range path {
		if r >= 'A' && r <= 'Z' {
			out.WriteByte('!')
			out.WriteRune(r + ('a' - 'A'))
			continue
		}
		out.WriteRune(r)
	}

	if _, err := url.Parse(out.String()); err != nil {
		return "", err
	}
	return out.String(), nil
}

// proxyFromEnv reads GOPROXY, taking the first entry that is a URL.
func proxyFromEnv() string {
	value := os.Getenv("GOPROXY")
	if value == "" {
		return DefaultProxy
	}

	for _, entry := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '|' }) {
		entry = strings.TrimSpace(entry)
		switch entry {
		case "", "direct", "off":
			continue
		}
		return strings.TrimSuffix(entry, "/")
	}

	// GOPROXY naming only direct or off is a build that reaches no proxy, and
	// so is this.
	return ""
}

// privateFromEnv reads the prefixes the toolchain keeps off the proxy.
func privateFromEnv() []string {
	var out []string

	for _, name := range []string{"GOPRIVATE", "GONOPROXY"} {
		for _, entry := range strings.Split(os.Getenv(name), ",") {
			if entry = strings.TrimSpace(entry); entry != "" {
				out = append(out, entry)
			}
		}
	}

	return out
}
