package main

import (
	"path/filepath"
	"testing"
)

// scopedRepo builds a module whose root package keeps its API while the sub
// package loses an exported symbol, with the kept state tagged. The verdict
// over the whole module is a minor; over the root package it is a patch.
func scopedRepo(t *testing.T) string {
	t.Helper()

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")

	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Greet greets.\nfunc Greet(name string) string { return name }\n")
	writeTestFile(t, filepath.Join(alpha, "sub", "sub.go"), "package sub\n\n// Helper helps.\nfunc Helper() string { return \"\" }\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "alpha: add Greet and sub.Helper")
	runGit(t, root, "tag", "alpha/v0.1.0")

	writeTestFile(t, filepath.Join(alpha, "sub", "sub.go"), "package sub\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: drop sub.Helper")

	return alpha
}

func TestScopeVerdictNarrowsToTheRootPackage(t *testing.T) {
	requireSplint(t)

	alpha := scopedRepo(t)
	whole, err := readVerdict(alpha, "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if whole.Release != releaseMinor || !whole.API.Breaking {
		t.Fatalf("readVerdict() = {Release: %q, Breaking: %v}, want a minor: the module loses sub.Helper", whole.Release, whole.API.Breaking)
	}

	scoped, err := scopeVerdict(whole, alpha)
	if err != nil {
		t.Fatalf("scopeVerdict() error: %v", err)
	}
	if scoped.Scope != scopeRoot {
		t.Errorf("scopeVerdict() Scope = %q, want %q", scoped.Scope, scopeRoot)
	}
	if scoped.API.Breaking {
		t.Error("scopeVerdict() left the diff breaking, want the subpackage removal dropped")
	}
	if len(scoped.API.Removed) != 0 {
		t.Errorf("scopeVerdict() API.Removed = %#v, want none: the root package lost nothing", scoped.API.Removed)
	}
	if scoped.Release != releasePatch || scoped.Version != "v0.1.1" {
		t.Errorf("scopeVerdict() = {Release: %q, Version: %q}, want {patch, v0.1.1}", scoped.Release, scoped.Version)
	}

	// The per-commit scans feed the commit table, so they are narrowed the
	// same way.
	for hash, diff := range scoped.CommitAPI {
		if len(diff.Removed) != 0 {
			t.Errorf("scopeVerdict() left commit %s with removals: %#v", hash, diff.Removed)
		}
	}
	for _, pkg := range scoped.Visibility.Packages {
		if pkg.Package != "./" {
			t.Errorf("scopeVerdict() kept the visibility of %s, want the root package alone", pkg.Package)
		}
	}
}

func TestScopeVerdictKeepsWhatTheRootTakesAway(t *testing.T) {
	requireSplint(t)

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")

	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Greet greets.\nfunc Greet(name string) string { return name }\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "alpha: add Greet")
	runGit(t, root, "tag", "alpha/v0.1.0")

	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: drop Greet")

	whole, err := readVerdict(alpha, "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	scoped, err := scopeVerdict(whole, alpha)
	if err != nil {
		t.Fatalf("scopeVerdict() error: %v", err)
	}
	if scoped.Release != releaseMinor || !scoped.API.Breaking {
		t.Errorf("scopeVerdict() = {Release: %q, Breaking: %v}, want a minor: the root package loses Greet", scoped.Release, scoped.API.Breaking)
	}
	if len(scoped.API.Removed) != 1 {
		t.Errorf("scopeVerdict() API.Removed = %#v, want Greet", scoped.API.Removed)
	}
}

func TestRootScopeArg(t *testing.T) {
	for arg, want := range map[string]bool{".": true, "./": true, "": false, "./...": false, "alpha": false} {
		if got := rootScopeArg(arg); got != want {
			t.Errorf("rootScopeArg(%q) = %v, want %v", arg, got, want)
		}
	}
}
