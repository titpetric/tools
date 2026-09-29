package main

import (
	"bytes"
	"strings"
	"testing"
)

// diagramModules is a workspace of two groups, where one module uses another in
// its own group and a third across groups, which is what the two diagrams draw
// differently. A group is the module path above the last segment, so the two
// tools share one and mig has its own.
func diagramModules() []moduleInfo {
	return []moduleInfo{
		{
			Name:        "github.com/titpetric/tools/worktree",
			Description: "worktree - Show workspace details",
			Uses:        []string{"github.com/titpetric/tools/splint", "github.com/go-bridget/mig"},
		},
		{Name: "github.com/titpetric/tools/splint", Description: "splint - A linting framework"},
		{Name: "github.com/go-bridget/mig"},
	}
}

func TestRenderD2(t *testing.T) {
	var out bytes.Buffer
	renderD2(&out, diagramModules())

	got := out.String()
	for _, want := range []string{
		"direction: down",
		// One container per group, labelled with the path above the module.
		"titpetric-tools: titpetric/tools {",
		"go-bridget: go-bridget {",
		// Each module carries a link into the repository holding it.
		"worktree.link: https://github.com/titpetric/tools/tree/main/worktree",
		// A description is written under the name, and loses the name in front
		// of its dash.
		`worktree: "worktree\nShow workspace details"`,
		// A dependency inside the group is drawn as an edge.
		"titpetric-tools.worktree <- titpetric-tools.splint",
		// One across groups is drawn as a note on the module being used, since
		// an edge between containers is what makes the diagram unreadable.
		"Used by worktree",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderD2() missing %q:\n%s", want, got)
		}
	}

	// The cross-group dependency is a note, not an edge.
	if strings.Contains(got, "titpetric-tools.worktree <- go-bridget.mig") {
		t.Errorf("renderD2() drew an edge across two containers:\n%s", got)
	}
}

// TestRenderD2WithNoModules checks an empty workspace still writes a document
// d2 can read, rather than nothing at all.
func TestRenderD2WithNoModules(t *testing.T) {
	var out bytes.Buffer
	renderD2(&out, nil)

	if got := out.String(); !strings.Contains(got, "direction: down") {
		t.Errorf("renderD2() of an empty workspace = %q", got)
	}
}

// TestD2Key checks a module path is turned into a key d2 accepts: the host is
// dropped and the separators that would read as nesting are replaced.
func TestD2Key(t *testing.T) {
	for in, want := range map[string]string{
		"github.com/titpetric/tools": "titpetric-tools",
		"charm.land/bubbletea":       "charm-land-bubbletea",
		"worktree":                   "worktree",
	} {
		if got := d2Key(in); got != want {
			t.Errorf("d2Key(%q) = %q, want %q", in, got, want)
		}
	}
}
