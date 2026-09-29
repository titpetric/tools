package main

import (
	"testing"
)

// TestModuleLink covers the two shapes a module path takes: a repository of
// its own, which is the path itself, and a module below one, which is a link
// into the subdirectory holding it.
func TestModuleLink(t *testing.T) {
	for in, want := range map[string]string{
		"github.com/titpetric/tools":                 "https://github.com/titpetric/tools",
		"github.com/titpetric/tools/worktree":        "https://github.com/titpetric/tools/tree/main/worktree",
		"github.com/titpetric/tools/worktree/config": "https://github.com/titpetric/tools/tree/main/worktree/config",
		// A path that is not on github is written as it stands.
		"gopkg.in/yaml.v3": "https://gopkg.in/yaml.v3",
	} {
		if got := moduleLink(in); got != want {
			t.Errorf("moduleLink(%q) = %q, want %q", in, got, want)
		}
	}
}
