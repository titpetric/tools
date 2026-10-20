// Package modproxy queries the Go module proxy about a dependency.
//
// It answers four things a document cannot: how big the module is, the
// publish date of the version in use, what the latest one is, and what that version
// itself requires. All four come from the proxy protocol without downloading
// anything: the size is the Content-Length of a HEAD on the zip, and the rest
// is one small document each.
//
// A report needs nothing from here. A machine with no network answers
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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/modfile"
)

// DefaultProxy is the proxy a client queries when GOPROXY says nothing.
const DefaultProxy = "https://proxy.golang.org"

// Info is what the proxy knows about one module version.
type Info struct {
	// Path and Version are the module asked about.
	Path    string `json:"Path"`
	Version string `json:"Version"`

	// Size is the module zip in bytes, the file a consumer downloads. Size
	// holds the compressed size: the proxy reports it without unpacking
	// anything, and unpacking every dependency to count the bytes inside costs
	// more than the number is worth.
	Size int64 `json:"Size,omitempty"`

	// Published is the tag time of the version in use, and Latest is the
	// newest version the proxy knows of.
	Published time.Time `json:"Published,omitzero"`
	Latest    string    `json:"Latest,omitempty"`

	// LatestPublished is the tag time of that newest version, and it says
	// whether a dependency is behind or simply finished.
	LatestPublished time.Time `json:"LatestPublished,omitzero"`

	// Requires are the module paths this version's own go.mod requires, in
	// order. A go.mod names what a build resolved to and not which module
	// required it, so this is the only place that records the shape of the
	// graph.
	Requires []string `json:"Requires,omitempty"`

	// Err is why the proxy could not answer, and is empty when it did.
	Err string `json:"Err,omitempty"`
}

// Behind reports a module pinned to something older than the latest.
func (i Info) Behind() bool {
	return i.Latest != "" && i.Version != "" && i.Latest != i.Version
}

// Age is the time since the publish of the version in use.
func (i Info) Age(now time.Time) time.Duration {
	if i.Published.IsZero() {
		return 0
	}
	return now.Sub(i.Published)
}

// Client queries a proxy about modules.
//
// A Client is safe for concurrent use and remembers every reply: a module
// version is immutable, so the same question has the same answer for the life
// of the process.
type Client struct {
	// Proxy is the base URL, and HTTP is what talks to it.
	Proxy string
	HTTP  *http.Client

	// Private are the module path prefixes not to query, the ones GOPRIVATE
	// and GONOPROXY name. A private module is nobody's business but
	// the person building it.
	Private []string

	// Sizes is what module versions weigh, kept between runs. A client
	// without one queries the proxy every time.
	Sizes *Cache

	// Offline keeps the client off the network. The client still reports what
	// the cache holds, and for a version the cache lacks it reports the size of
	// the module it belongs to, so a report draws on what earlier runs
	// learned and never starts from nothing.
	Offline bool

	mu    sync.Mutex
	known map[string]Info
}

// New returns a client reading GOPROXY, GOPRIVATE and GONOPROXY the way the
// toolchain reads them.
//
// New reads a GOPROXY listing more than one proxy down to the first real URL:
// the fallbacks are what the toolchain does when a download fails, and this
// client puts one question that either answers or does not.
func New() *Client {
	return &Client{
		Proxy:   proxyFromEnv(),
		HTTP:    &http.Client{Timeout: 10 * time.Second},
		Private: privateFromEnv(),
		Sizes:   OpenCache(DefaultCachePath()),
		known:   map[string]Info{},
	}
}

// Flush writes what the client learned about module sizes, and a caller runs
// it once done with its queries. A cache write failing does not fail the
// report the cache served.
func (c *Client) Flush() error {
	return c.Sizes.Save()
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
	// This method fills the map and New does not, so a client written out as
	// a literal is a client that works.
	if c.known == nil {
		c.known = map[string]Info{}
	}
	c.known[key] = info
	c.mu.Unlock()

	return info
}

// LookupAll queries many modules at once, as a report needs: a tree of thirty
// dependencies queried one at a time is thirty round trips end to end.
func (c *Client) LookupAll(ctx context.Context, modules map[string]string) map[string]Info {
	return each(ctx, modules, c.Lookup)
}

// SizeAll is what a set of module versions weigh, and nothing else about them.
//
// SizeAll puts one question and skips the other three. A caller queries a
// version whose source go.sum records to put a number on a copy the build
// downloads, and its publish date or its requirements say nothing about that.
func (c *Client) SizeAll(ctx context.Context, modules map[string]string) map[string]int64 {
	return each(ctx, modules, c.SizeOf)
}

// SizeOf is what one version weighs, from the cache when the cache holds it
// and from the proxy otherwise.
func (c *Client) SizeOf(ctx context.Context, path, version string) int64 {
	if size, known := c.Sizes.Size(path, version); known {
		return size
	}

	if c.Offline || c.Proxy == "" || c.private(path) {
		return 0
	}
	escaped, err := escapePath(path)
	if err != nil {
		return 0
	}

	size, err := c.size(ctx, escaped, version)
	if err != nil {
		return 0
	}

	c.Sizes.Add(path, version, size)
	return size
}

// each asks one question of every module, a few at a time.
//
// each keys the results by the queried module, so a failed query still leaves
// an entry: a caller reading the map gets the zero value and can tell the
// query ran.
func each[T any](ctx context.Context, modules map[string]string, ask func(context.Context, string, string) T) map[string]T {
	const workers = 8

	type question struct{ path, version string }
	type answer struct {
		path string
		of   T
	}

	queue := make(chan question)
	out := make(chan answer)

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for q := range queue {
				out <- answer{path: q.path, of: ask(ctx, q.path, q.version)}
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

	answers := make(map[string]T, len(modules))
	for one := range out {
		answers[one.path] = one.of
	}
	return answers
}

// ask is what the proxy knows about one module version: the size from the
// cache or the proxy, and the rest from the proxy alone.
func (c *Client) ask(ctx context.Context, path, version string) Info {
	info := Info{Path: path, Version: version}

	// A size the cache holds needs no query, and an offline run reports from
	// the cache.
	if size, known := c.Sizes.Size(path, version); known {
		info.Size = size
	}

	if c.Offline || c.Proxy == "" || c.private(path) {
		info.Err = "not asked: the run is offline, the module is private, or no proxy is configured"
		return info
	}

	escaped, err := escapePath(path)
	if err != nil {
		info.Err = err.Error()
		return info
	}

	if info.Size == 0 {
		if size, err := c.size(ctx, escaped, version); err != nil {
			info.Err = err.Error()
		} else {
			info.Size = size
			c.Sizes.Add(path, version, size)
		}
	}

	if published, err := c.published(ctx, escaped, version); err == nil {
		info.Published = published
	}

	if latest, published, err := c.latest(ctx, escaped); err == nil {
		info.Latest = latest
		info.LatestPublished = published
	}

	if requires, err := c.requires(ctx, escaped, version); err == nil {
		info.Requires = requires
	}

	return info
}

// requires reads the go.mod of a version and returns what it requires.
//
// That go.mod is the one document that says which module requires which. The
// go.mod of the module under report names every module its build resolves to
// and nothing about which dependency dragged which one in, so only a query to
// each dependency for its requirements shows the reach of a dependency.
func (c *Client) requires(ctx context.Context, escaped, version string) ([]string, error) {
	response, err := c.do(ctx, http.MethodGet, fmt.Sprintf("%s/%s/@v/%s.mod", c.Proxy, escaped, version))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	parsed, err := modfile.Parse("go.mod", body, nil)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(parsed.Require))
	for _, require := range parsed.Require {
		out = append(out, require.Mod.Path)
	}
	sort.Strings(out)

	return out, nil
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

// published is the tag time of a version.
func (c *Client) published(ctx context.Context, escaped, version string) (time.Time, error) {
	var out struct{ Time time.Time }
	err := c.json(ctx, fmt.Sprintf("%s/%s/@v/%s.info", c.Proxy, escaped, version), &out)
	return out.Time, err
}

// latest is the newest version the proxy knows of, and its tag time.
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
	if c.Offline {
		return nil, fmt.Errorf("the run is offline")
	}

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

// private reports a module the client must not query the proxy about.
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
// upper case letter written as a bang and its lower case: proxies serve the
// protocol off case insensitive filesystems, and two modules differing only in
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
