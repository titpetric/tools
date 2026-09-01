package modproxy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cacheFile is a cache in a directory of its own, which is what a save writes
// into and what a reopen reads back.
func cacheFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "sizes.yml")
}

// TestCacheRoundTrip covers the point of the file: what one run learned is
// what the next run answers with, without asking anybody.
func TestCacheRoundTrip(t *testing.T) {
	path := cacheFile(t)

	cache := OpenCache(path)
	cache.Add("example.com/x", "v1.0.0", 1000)
	cache.Add("example.com/x", "v1.1.0", 2000)
	cache.Add("example.com/y", "v0.1.0", 500)

	if err := cache.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	read := OpenCache(path)
	if got, known := read.Size("example.com/x", "v1.0.0"); !known || got != 1000 {
		t.Errorf("Size() = %d, %v, want 1000", got, known)
	}

	// A version the file does not hold is answered with the size of the
	// module, which is the mean of the ones it does.
	got, known := read.Size("example.com/x", "v9.9.9")
	if !known || got != 1500 {
		t.Errorf("Size() of an unknown version = %d, %v, want the mean 1500", got, known)
	}

	if _, known := read.Size("example.com/nothing", "v1.0.0"); known {
		t.Error("Size() answered for a module the cache has never seen")
	}
}

// TestCacheSaveIsAtomic covers how the file is written: beside itself and
// renamed, so a reader sees one file or the other.
func TestCacheSaveIsAtomic(t *testing.T) {
	path := cacheFile(t)

	cache := OpenCache(path)
	cache.Add("example.com/x", "v1.0.0", 1000)
	if err := cache.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Error("Save() left the file it wrote through")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the cache: %v", err)
	}
	// The shape is the module, its size, and the size of each version.
	for _, want := range []string{"example.com/x:", "size: 1000", "versions:", "v1.0.0: 1000"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("the file does not hold %q:\n%s", want, data)
		}
	}
}

// TestCacheSaveWritesNothingNew covers the run that learned nothing: the file
// is left where it is rather than rewritten with what it already said.
func TestCacheSaveWritesNothingNew(t *testing.T) {
	path := cacheFile(t)

	cache := OpenCache(path)
	cache.Add("example.com/x", "v1.0.0", 1000)
	if err := cache.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	// The same answer again, and a size of nothing, which is what the proxy
	// reports when it could not be reached.
	read := OpenCache(path)
	read.Add("example.com/x", "v1.0.0", 1000)
	read.Add("example.com/y", "v1.0.0", 0)
	if err := read.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("Save() rewrote a cache that learned nothing")
	}
	if len(read.Modules()) != 1 {
		t.Errorf("Modules() = %v, want the one that has a size", read.Modules())
	}
}

// TestOpenCacheOnRubbish covers the file nobody can read. A cache is an
// optimisation, so it answers nothing and is written over.
func TestOpenCacheOnRubbish(t *testing.T) {
	path := cacheFile(t)
	if err := os.WriteFile(path, []byte("\tnot: [yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	cache := OpenCache(path)
	if len(cache.Modules()) != 0 {
		t.Errorf("OpenCache() read %v out of rubbish", cache.Modules())
	}

	cache.Add("example.com/x", "v1.0.0", 1000)
	if err := cache.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if got, _ := OpenCache(path).Size("example.com/x", "v1.0.0"); got != 1000 {
		t.Errorf("Size() = %d after writing over an unreadable cache", got)
	}
}

// TestCacheWithoutPath covers the client that was given nowhere to write: it
// answers from memory and saves nothing.
func TestCacheWithoutPath(t *testing.T) {
	cache := OpenCache("")
	cache.Add("example.com/x", "v1.0.0", 1000)

	if got, known := cache.Size("example.com/x", "v1.0.0"); !known || got != 1000 {
		t.Errorf("Size() = %d, %v", got, known)
	}
	if err := cache.Save(); err != nil {
		t.Errorf("Save() error = %v, want nothing written and no error", err)
	}
}

// TestClientOffline covers the run that asks nobody: the size comes out of the
// cache, and the questions only the proxy can answer are left blank.
func TestClientOffline(t *testing.T) {
	calls := 0
	server := proxy(t, &calls)
	defer server.Close()

	cache := OpenCache("")
	cache.Add("example.com/x", "v1.2.0", 4096)

	client := &Client{
		Proxy:   server.URL,
		HTTP:    server.Client(),
		Sizes:   cache,
		Offline: true,
		known:   map[string]Info{},
	}

	got := client.Lookup(context.Background(), "example.com/x", "v1.2.0")
	if got.Size != 4096 {
		t.Errorf("Size = %d, want the cached 4096", got.Size)
	}
	if got.Latest != "" || len(got.Requires) != 0 {
		t.Errorf("an offline lookup answered %#v", got)
	}
	if calls != 0 {
		t.Errorf("the proxy was called %d times on an offline run", calls)
	}
}

// TestClientCachesSizes covers the client filling the cache: the second run
// over the same module asks nobody.
func TestClientCachesSizes(t *testing.T) {
	calls := 0
	server := proxy(t, &calls)
	defer server.Close()

	cache := OpenCache("")
	client := &Client{Proxy: server.URL, HTTP: server.Client(), Sizes: cache, known: map[string]Info{}}

	if got := client.SizeOf(context.Background(), "example.com/x", "v1.2.0"); got != 111305 {
		t.Fatalf("SizeOf() = %d", got)
	}
	asked := calls

	if got := client.SizeOf(context.Background(), "example.com/x", "v1.2.0"); got != 111305 {
		t.Errorf("SizeOf() = %d on the second ask", got)
	}
	if calls != asked {
		t.Errorf("the cached size was asked for again: %d calls became %d", asked, calls)
	}
}
