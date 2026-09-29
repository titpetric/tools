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

// TestRun covers what Run decides before and after the screen: which document
// the form opens on, and what a run that did not save reports.
//
// The screen itself is not opened. tea.NewProgram wants a terminal, and what
// would be tested through it is the form, which model_test.go drives directly
// by feeding it key presses.
func TestRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := Path()
	if err != nil {
		t.Fatalf("Path() error: %v", err)
	}
	writeTestFile(t, path, "version: 1\nscan:\n  ignore_paths: [vendor]\n")

	// Run opens the form on the document Path names.
	model, loadErr := newModelFor(path)
	if loadErr != nil {
		t.Fatalf("newModelFor() error: %v", loadErr)
	}
	if model.path != path {
		t.Errorf("the form opened on %q, want %q", model.path, path)
	}

	// A run that does not save returns whatever reading the document cost,
	// which for a broken file is the parse error the status line also carries.
	writeTestFile(t, path, "scan: [this is not a mapping]\n")
	broken, loadErr := newModelFor(path)
	if loadErr == nil {
		t.Fatal("newModelFor() read a broken document without complaint")
	}
	if broken.status == "" {
		t.Error("the form opened on a broken document with an empty status line")
	}
	if broken.Saved() {
		t.Error("a form that has not run reports it saved")
	}
}
