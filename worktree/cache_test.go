package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain points the user cache directory at a temporary one, so a test run
// neither reads the models a real run left behind nor leaves any of its own.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "worktree-cache-test-")
	if err == nil {
		defer os.RemoveAll(dir)
		os.Setenv("XDG_CACHE_HOME", dir)
	}
	m.Run()
}

// cacheHome points the user cache directory at one of the test's own, and
// returns the directory entries are written below.
func cacheHome(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	root, err := os.UserCacheDir()
	if err != nil {
		t.Fatalf("os.UserCacheDir() error: %v", err)
	}
	return filepath.Join(root, "worktree")
}

// cacheEntries returns the model files the cache holds.
func cacheEntries(t *testing.T, dir string) []string {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(dir, "verdict-*.json"))
	if err != nil {
		t.Fatalf("filepath.Glob() error: %v", err)
	}
	return matches
}

func TestVerdictCacheKeepsTheModelOfACommit(t *testing.T) {
	requireGoFsck(t)

	dir := cacheHome(t)
	alpha := verdictRepo(t)

	if _, err := readVerdict(alpha, "", "", true); err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	written := cacheEntries(t, dir)
	if len(written) == 0 {
		t.Fatalf("readVerdict() left nothing under %s", dir)
	}

	// A second run reads the models back rather than writing new ones: the
	// commits it covers have not moved, and neither has the tool that read
	// them.
	stamps := make(map[string]int64, len(written))
	for _, entry := range written {
		info, err := os.Stat(entry)
		if err != nil {
			t.Fatalf("os.Stat() error: %v", err)
		}
		stamps[entry] = info.ModTime().UnixNano()
	}

	if _, err := readVerdict(alpha, "", "", true); err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	again := cacheEntries(t, dir)
	if len(again) != len(written) {
		t.Errorf("readVerdict() rewrote the cache: %d entries, want the %d it already held", len(again), len(written))
	}
	for _, entry := range again {
		info, err := os.Stat(entry)
		if err != nil {
			t.Fatalf("os.Stat() error: %v", err)
		}
		if stamp, ok := stamps[entry]; !ok || info.ModTime().UnixNano() != stamp {
			t.Errorf("readVerdict() wrote %s again, want it read back", filepath.Base(entry))
		}
	}
}

func TestVerdictCacheIsNotWrittenWithoutIt(t *testing.T) {
	requireGoFsck(t)

	dir := cacheHome(t)
	if _, err := readVerdict(verdictRepo(t), "", "", false); err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}

	if entries := cacheEntries(t, dir); len(entries) > 0 {
		t.Errorf("readVerdict() wrote %v with the cache turned off", entries)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Errorf("readVerdict() made %s with the cache turned off", dir)
	}
}

func TestVerdictCacheKeysOnTheCommitATagNames(t *testing.T) {
	dir := cacheHome(t)
	alpha := verdictRepo(t)

	cache := openVerdictCache(true)
	if cache.dir != dir {
		t.Fatalf("openVerdictCache() dir = %q, want %q", cache.dir, dir)
	}

	// A tag is read as the commit it names, so moving the tag to another commit
	// reads another entry, and naming the same commit twice reads one.
	tagged := cache.path(alpha, "alpha/v0.2.0")
	if tagged == "" {
		t.Fatal("path() named no entry for a tag the repository carries")
	}
	if head := cache.path(alpha, "HEAD"); head != tagged {
		t.Errorf("path(HEAD) = %q, want the entry of the commit it names, %q", head, tagged)
	}
	if older := cache.path(alpha, "alpha/v0.1.0"); older == tagged {
		t.Error("path() gave two commits the same entry")
	}

	// A revision the repository does not carry has no commit to key on, and is
	// read without the cache.
	if missing := cache.path(alpha, "alpha/v9.9.9"); missing != "" {
		t.Errorf("path() named %q for a revision that does not exist", missing)
	}
}

func TestVerdictCacheHoldsNothingWhenTurnedOff(t *testing.T) {
	cacheHome(t)

	cache := openVerdictCache(false)
	if cache.dir != "" {
		t.Errorf("openVerdictCache(false) dir = %q, want none", cache.dir)
	}
	if path := cache.path(".", "HEAD"); path != "" {
		t.Errorf("path() = %q with the cache turned off, want none", path)
	}
	if cache.load("", filepath.Join(t.TempDir(), "model.json")) {
		t.Error("load() read an entry with the cache turned off")
	}
	// Storing into a cache that holds nothing is a run without one, not a
	// failure.
	cache.store("", filepath.Join(t.TempDir(), "model.json"))
}
