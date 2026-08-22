package config

import (
	"reflect"
	"testing"
)

func TestScanIgnored(t *testing.T) {
	scan := Scan{IgnorePaths: []string{"node_modules", "vendor"}}
	for _, name := range []string{"node_modules", "vendor"} {
		if !scan.Ignored(name) {
			t.Fatalf("Ignored(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"", "vendored", "src"} {
		if scan.Ignored(name) {
			t.Fatalf("Ignored(%q) = true, want false", name)
		}
	}

	if (Scan{}).Ignored("vendor") {
		t.Fatal("Ignored() with no ignore paths = true, want false")
	}
}

// TestDefaultEnablesScanning checks the built-in defaults keep the behaviour
// the tool had before it was configurable: .gitignore honoured, git
// repositories listed, the three root markers, and no extra ignores.
func TestDefaultEnablesScanning(t *testing.T) {
	cfg := Default()

	if cfg.Version != Version {
		t.Fatalf("Default().Version = %d, want %d", cfg.Version, Version)
	}
	if !cfg.Scan.EnableGitignore {
		t.Fatal("Default().Scan.EnableGitignore = false, want true")
	}
	if !cfg.Scan.EnableGitRepos {
		t.Fatal("Default().Scan.EnableGitRepos = false, want true")
	}
	if len(cfg.Scan.IgnorePaths) != 0 {
		t.Fatalf("Default().Scan.IgnorePaths = %v, want none", cfg.Scan.IgnorePaths)
	}
	want := []string{"go.work", "go.mod", ".git"}
	if !reflect.DeepEqual(cfg.Scan.RootMarkers, want) {
		t.Fatalf("Default().Scan.RootMarkers = %v, want %v", cfg.Scan.RootMarkers, want)
	}
}
