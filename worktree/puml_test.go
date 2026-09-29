package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderPUML(t *testing.T) {
	var out bytes.Buffer
	renderPUML(&out, diagramModules())

	got := out.String()
	for _, want := range []string{
		"@startuml",
		"@enduml",
		// One package per group, labelled with the path above the module.
		`package "titpetric/tools"`,
		`package "go-bridget"`,
		// A description is written as the bold name over the rest of it.
		`<b>worktree</b>\nShow workspace details`,
		// Each component links into the repository holding it.
		"[[https://github.com/titpetric/tools/tree/main/worktree",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderPUML() missing %q:\n%s", want, got)
		}
	}

	// Every dependency is an arrow, whether or not it crosses a package: the
	// plantuml layout draws those without becoming unreadable.
	for _, want := range []string{
		pumlAlias("github.com/titpetric/tools/worktree"),
		pumlAlias("github.com/titpetric/tools/splint"),
		pumlAlias("github.com/go-bridget/mig"),
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderPUML() did not name the alias %q:\n%s", want, got)
		}
	}
}

// TestRenderPUMLWithNoModules checks an empty workspace still writes a diagram
// plantuml can read.
func TestRenderPUMLWithNoModules(t *testing.T) {
	var out bytes.Buffer
	renderPUML(&out, nil)

	got := out.String()
	if !strings.HasPrefix(got, "@startuml") || !strings.Contains(got, "@enduml") {
		t.Errorf("renderPUML() of an empty workspace = %q", got)
	}
}

// TestPumlAlias checks a module path becomes an identifier plantuml accepts:
// every separator is replaced, since a dot or a slash in an alias is read as
// syntax.
func TestPumlAlias(t *testing.T) {
	for in, want := range map[string]string{
		"github.com/titpetric/tools": "github_com_titpetric_tools",
		"charm.land/bubbletea/v2":    "charm_land_bubbletea_v2",
		"go-bridget/mig":             "go_bridget_mig",
	} {
		if got := pumlAlias(in); got != want {
			t.Errorf("pumlAlias(%q) = %q, want %q", in, got, want)
		}
	}
}
