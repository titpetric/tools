package modproxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// proxy stands in for the real one, answering the three questions a lookup
// asks and counting how often it was asked.
func proxy(t *testing.T, calls *int) *httptest.Server {
	t.Helper()

	published := time.Date(2025, 8, 26, 17, 56, 1, 0, time.UTC)
	latest := time.Date(2026, 8, 20, 9, 37, 52, 0, time.UTC)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++

		switch {
		case r.Method == http.MethodHead:
			// The size is a header on the zip, so the zip is never fetched.
			w.Header().Set("Content-Length", strconv.Itoa(111305))
			w.WriteHeader(http.StatusOK)

		case r.URL.Path == "/example.com/x/@latest":
			json.NewEncoder(w).Encode(map[string]any{"Version": "v1.4.0", "Time": latest})

		default:
			json.NewEncoder(w).Encode(map[string]any{"Version": "v1.2.0", "Time": published})
		}
	}))
}

func TestClient_Lookup(t *testing.T) {
	calls := 0
	server := proxy(t, &calls)
	defer server.Close()

	client := &Client{Proxy: server.URL, HTTP: server.Client(), known: map[string]Info{}}
	got := client.Lookup(context.Background(), "example.com/x", "v1.2.0")

	if got.Err != "" {
		t.Fatalf("Lookup() err = %q", got.Err)
	}
	if got.Size != 111305 {
		t.Errorf("Size = %d, want the Content-Length", got.Size)
	}
	if got.Latest != "v1.4.0" || !got.Behind() {
		t.Errorf("Latest = %q, Behind = %v", got.Latest, got.Behind())
	}
	if got.Published.Year() != 2025 {
		t.Errorf("Published = %v", got.Published)
	}
	if got.Age(time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)) <= 0 {
		t.Error("Age() of a published version is not positive")
	}

	// A module version is immutable, so the same question is asked once.
	asked := calls
	client.Lookup(context.Background(), "example.com/x", "v1.2.0")
	if calls != asked {
		t.Errorf("Lookup() asked again: %d calls became %d", asked, calls)
	}
}

// TestClient_LookupUnreachable covers the machine with no network. A report is
// worth having without the proxy, so a failure is recorded and not returned.
func TestClient_LookupUnreachable(t *testing.T) {
	client := &Client{
		Proxy: "http://127.0.0.1:1",
		HTTP:  &http.Client{Timeout: 100 * time.Millisecond},
		known: map[string]Info{},
	}

	got := client.Lookup(context.Background(), "example.com/x", "v1.2.0")
	if got.Err == "" {
		t.Error("Lookup() reported no error against a proxy that is not there")
	}
	if got.Path != "example.com/x" || got.Version != "v1.2.0" {
		t.Errorf("Lookup() lost what it was asked: %#v", got)
	}
	if got.Behind() {
		t.Error("Behind() said a module with no known latest is behind")
	}
}

func TestClient_LookupPrivate(t *testing.T) {
	calls := 0
	server := proxy(t, &calls)
	defer server.Close()

	client := &Client{
		Proxy:   server.URL,
		HTTP:    server.Client(),
		Private: []string{"example.com/private"},
		known:   map[string]Info{},
	}

	got := client.Lookup(context.Background(), "example.com/private/x", "v1.0.0")
	if got.Err == "" {
		t.Error("Lookup() asked about a private module")
	}
	if calls != 0 {
		t.Errorf("the proxy was called %d times for a private module", calls)
	}
}

func TestClient_LookupAll(t *testing.T) {
	calls := 0
	server := proxy(t, &calls)
	defer server.Close()

	client := &Client{Proxy: server.URL, HTTP: server.Client(), known: map[string]Info{}}
	got := client.LookupAll(context.Background(), map[string]string{
		"example.com/x": "v1.2.0",
		"example.com/y": "v0.5.0",
		"example.com/z": "v2.0.0",
	})

	if len(got) != 3 {
		t.Fatalf("LookupAll() = %d answers, want 3", len(got))
	}
	for path, info := range got {
		if info.Path != path {
			t.Errorf("answer keyed %q holds %q", path, info.Path)
		}
		if info.Size != 111305 {
			t.Errorf("%s: Size = %d", path, info.Size)
		}
	}
}

// TestEscapePath covers the spelling the protocol uses: the proxy is served
// off case insensitive filesystems, so two modules differing only in case
// would otherwise be one file.
func TestEscapePath(t *testing.T) {
	tests := map[string]string{
		"github.com/BurntSushi/toml": "github.com/!burnt!sushi/toml",
		"example.com/x":              "example.com/x",
		"example.com/X/v2":           "example.com/!x/v2",
	}

	for path, want := range tests {
		got, err := escapePath(path)
		if err != nil {
			t.Fatalf("escapePath(%q) error = %v", path, err)
		}
		if got != want {
			t.Errorf("escapePath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestProxyFromEnv(t *testing.T) {
	tests := map[string]string{
		"":                               DefaultProxy,
		"https://example.com/mod":        "https://example.com/mod",
		"https://example.com/mod/":       "https://example.com/mod",
		"https://one.example.com,direct": "https://one.example.com",
		// A build that reaches no proxy asks nobody.
		"off":    "",
		"direct": "",
	}

	for value, want := range tests {
		t.Setenv("GOPROXY", value)
		if got := proxyFromEnv(); got != want {
			t.Errorf("GOPROXY=%q gives %q, want %q", value, got, want)
		}
	}
}
