package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNewModelForReadsTheDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worktree.yml")
	writeTestFile(t, path, "scan:\n  enable_git_repos: true\n")

	model, err := newModelFor(path)
	if err != nil {
		t.Fatalf("newModelFor() error: %v", err)
	}
	if model.config.Scan.EnableGitignore {
		t.Fatal("the screen opened on the defaults rather than the document")
	}
	if model.status != "" {
		t.Fatalf("status = %q, want none", model.status)
	}
}

// TestNewModelForOpensOnDefaults checks a document that cannot be parsed still
// opens the screen, which is where it gets fixed, with the reason showing.
func TestNewModelForOpensOnDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worktree.yml")
	writeTestFile(t, path, "scan: [not a mapping\n")

	model, err := newModelFor(path)
	if err == nil {
		t.Fatal("newModelFor() with a broken document, want an error")
	}
	if !model.config.Scan.EnableGitignore {
		t.Fatal("the screen did not open on the built-in defaults")
	}
	if !strings.Contains(model.status, "parse config") {
		t.Fatalf("status = %q, want the parse error", model.status)
	}
	if model.saved {
		t.Fatal("the screen reports it saved before it ran")
	}
}

func TestNewModelForMissingDocument(t *testing.T) {
	model, err := newModelFor(filepath.Join(t.TempDir(), "worktree.yml"))
	if err != nil {
		t.Fatalf("newModelFor() error: %v", err)
	}
	if !model.config.Scan.EnableGitignore {
		t.Fatal("the screen did not open on the built-in defaults")
	}
	if model.status != "" {
		t.Fatalf("status = %q, want none for a document that simply does not exist", model.status)
	}
}
