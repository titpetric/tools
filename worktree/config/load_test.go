package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeTestFile writes a file, creating the directories above it.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPathIsBelowHome(t *testing.T) {
	t.Setenv("HOME", "/home/test")

	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/home/test", ".config", "worktree.yml")
	if got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

func TestLoadFileMissingReturnsDefaults(t *testing.T) {
	got, err := LoadFile(filepath.Join(t.TempDir(), "worktree.yml"))
	if err != nil {
		t.Fatalf("LoadFile() error: %v", err)
	}
	if !reflect.DeepEqual(got, Default()) {
		t.Fatalf("LoadFile() = %#v, want the defaults %#v", got, Default())
	}
}

// TestLoadFileDoesNotMergeDefaults checks the documented rule: an existing
// file is the whole configuration, so a setting it does not name reads as off
// rather than falling back to the built-in value.
func TestLoadFileDoesNotMergeDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worktree.yml")
	writeTestFile(t, path, "scan:\n  enable_git_repos: true\n")

	got, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error: %v", err)
	}
	if !got.Scan.EnableGitRepos {
		t.Fatal("Scan.EnableGitRepos = false, want the value the file set")
	}
	if got.Scan.EnableGitignore {
		t.Fatal("Scan.EnableGitignore = true, want the defaults not to be merged in")
	}
	if len(got.Scan.RootMarkers) != 0 {
		t.Fatalf("Scan.RootMarkers = %v, want the defaults not to be merged in", got.Scan.RootMarkers)
	}
}

func TestLoadFileRejectsNewerVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worktree.yml")
	writeTestFile(t, path, "version: 99\n")

	if _, err := LoadFile(path); err == nil {
		t.Fatal("LoadFile() with a newer document version, want an error")
	}
}

func TestLoadFileReportsBrokenDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worktree.yml")
	writeTestFile(t, path, "scan: [this is not a mapping\n")

	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("LoadFile() with a broken document, want an error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("LoadFile() error = %v, want it to name %s", err, path)
	}
}

// TestSaveFileWritesEveryKey checks the safety valve for the no merge rule:
// the file the setup screen writes names every setting, including the ones
// left off, so a round trip cannot silently drop one.
func TestSaveFileWritesEveryKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "worktree.yml")
	cfg := &Config{}

	if err := SaveFile(path, cfg); err != nil {
		t.Fatalf("SaveFile() error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, key := range []string{
		"version:",
		"scan:",
		"enable_gitignore:",
		"enable_git_repos:",
		"ignore_paths:",
		"root_markers:",
	} {
		if !strings.Contains(got, key) {
			t.Fatalf("SaveFile() output is missing %q:\n%s", key, got)
		}
	}
}

func TestSaveFileRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worktree.yml")
	want := &Config{
		Version: Version,
		Scan: Scan{
			EnableGitRepos: true,
			IgnorePaths:    []string{"node_modules"},
			RootMarkers:    []string{"go.work"},
		},
	}

	if err := SaveFile(path, want); err != nil {
		t.Fatalf("SaveFile() error: %v", err)
	}
	got, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadFile() = %#v, want %#v", got, want)
	}
}

// TestEncodeStampsVersion checks a document written by an older build, or by
// hand without one, comes back carrying the version this build writes.
func TestEncodeStampsVersion(t *testing.T) {
	cfg := &Config{}
	data, err := Encode(cfg)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}
	if cfg.Version != 0 {
		t.Fatalf("Encode() changed its argument, Version = %d", cfg.Version)
	}
	got, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if got.Version != Version {
		t.Fatalf("Parse(Encode()) version = %d, want %d", got.Version, Version)
	}
}

// TestLoad and TestSave cover the pair that go through Path: Load reads the
// document below the home directory, and Save writes it there, creating the
// .config directory it lives in.
func TestLoad(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// No document yet, so the built-in defaults answer.
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.Scan.EnableGitignore {
		t.Error("Load() without a document did not return the defaults")
	}

	writeTestFile(t, filepath.Join(home, ".config", "worktree.yml"), "version: 1\nscan:\n  ignore_paths: [vendor]\n")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !reflect.DeepEqual(cfg.Scan.IgnorePaths, []string{"vendor"}) {
		t.Errorf("Load() = %#v, want the document on disk", cfg.Scan)
	}
	// The file is the whole configuration, so a setting it does not name is off.
	if cfg.Scan.EnableGitignore {
		t.Error("Load() filled a missing setting in from the defaults")
	}
}

func TestSave(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	want := &Config{Scan: Scan{
		EnableGitRepos: true,
		IgnorePaths:    []string{"node_modules"},
		RootMarkers:    []string{"go.work"},
	}}
	if err := Save(want); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	path := filepath.Join(home, ".config", "worktree.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Save() wrote nothing to %s: %v", path, err)
	}
	if !strings.Contains(string(data), "node_modules") {
		t.Errorf("Save() left the setting out of %s:\n%s", path, data)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() after Save() error: %v", err)
	}
	if !reflect.DeepEqual(got.Scan, want.Scan) {
		t.Errorf("Load() after Save() = %#v, want %#v", got.Scan, want.Scan)
	}
}
