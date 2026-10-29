package test

import (
	"strings"
	"testing"
)

// TestListSelectsChangedFunction covers the common case: a function body moved
// and its test runs.
func TestListSelectsChangedFunction(t *testing.T) {
	dir := changedAdd(t)
	write(t, dir, "p.go", "package t\n\nfunc Add(a, b int) int {\n\treturn b + a\n}\n")

	got := list(t, Options{SourcePath: dir, Pattern: "./..."})
	if !strings.Contains(got, "TestAdd\n") {
		t.Fatalf("the change selected no test:\n%s", got)
	}
}

// TestListIgnoresDocChange covers what the run leaves alone: a comment above a
// function and a type's fields are not function bodies.
func TestListIgnoresDocChange(t *testing.T) {
	dir := changedAdd(t)
	write(t, dir, "p.go", "package t\n\n// Add sums two numbers.\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\ntype Pair struct {\n\tA int\n\tB int\n}\n")
	commit(t, dir, "comment and type")
	write(t, dir, "p.go", "package t\n\n// Add adds two numbers.\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\ntype Pair struct {\n\tA int\n\tB int\n\tC int\n}\n")

	if got := list(t, Options{SourcePath: dir, Pattern: "./..."}); strings.TrimSpace(got) != "" {
		t.Fatalf("a doc and a type change selected tests:\n%s", got)
	}
}
