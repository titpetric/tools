package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/titpetric/tools/worktree/components"
)

func TestRenderDependencyMatrix(t *testing.T) {
	modules := []moduleInfo{
		{
			Name:     "example.com/service",
			Uses:     []string{"example.com/library"},
			GitState: &components.Git{Ahead: 1},
		},
		{Name: "example.com/library"},
		{
			Name:     "example.com/client",
			Uses:     []string{"example.com/library", "example.com/service"},
			GitState: &components.Git{DiffLines: []string{"go.mod +1/-1"}},
		},
		{Name: "example.com/tool", GitState: &components.Git{UntrackedFiles: []string{"PLAN.md"}}},
	}

	var output bytes.Buffer
	refs := versionRefs{
		"example.com/service": {"example.com/library": "v1.0.0"},
		"example.com/client":  {"example.com/library": "v2.0.0", "example.com/service": "v1.0.0"},
	}
	tags := latestTags{
		"example.com/library": "v2.0.0",
		"example.com/service": "v1.0.0",
	}
	renderDependencyMatrix(&output, modules, refs, tags, true)

	want := "" +
		"╭──────────────┬─────────┬─────────╮\n" +
		"│ Project      │ service │ library │\n" +
		"├──────────────┼─────────┼─────────┤\n" +
		"│ service (+1) │         │ ▲*      │\n" +
		"│ client *     │ ▲       │ ▲       │\n" +
		"│ tool *       │         │         │\n" +
		"╰──────────────┴─────────┴─────────╯\n" +
		"1 ahead, 2 with local changes, 1 deps out of date.\n"
	if got := ansi.Strip(output.String()); got != want {
		t.Fatalf("renderDependencyMatrix() =\n%s\nwant:\n%s", got, want)
	}
	if got := output.String(); !strings.Contains(got, components.ColorGreen+"▲") || !strings.Contains(got, components.ColorYellow+"▲*") {
		t.Fatalf("renderDependencyMatrix() did not color current and outdated dependencies: %q", got)
	}
	if got := output.String(); !strings.Contains(got, components.ColorSeparator+"(+1)") || !strings.Contains(got, components.ColorDarkOrange+"*") {
		t.Fatalf("renderDependencyMatrix() did not show project Git state: %q", got)
	}
	if got := output.String(); !strings.Contains(got, components.ColorHeader+"1 ahead, 2 with local changes, 1 deps out of date.") {
		t.Fatalf("renderDependencyMatrix() did not style summary: %q", got)
	}
}

func TestRenderDependencyMatrixMinWidth(t *testing.T) {
	modules := []moduleInfo{
		{Name: "example.com/app", Uses: []string{"example.com/db"}},
		{Name: "example.com/db"},
	}

	var output bytes.Buffer
	renderDependencyMatrix(&output, modules, nil, nil, true)

	want := "" +
		"╭─────────┬─────╮\n" +
		"│ Project │ db  │\n" +
		"├─────────┼─────┤\n" +
		"│ app     │ ▲   │\n" +
		"╰─────────┴─────╯\n" +
		"0 ahead, 0 with local changes, 0 deps out of date.\n"
	if got := ansi.Strip(output.String()); got != want {
		t.Fatalf("renderDependencyMatrix() =\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderDependencyMatrixMarkdown(t *testing.T) {
	modules := []moduleInfo{
		{Name: "example.com/app", Uses: []string{"example.com/lib"}},
		{Name: "example.com/lib"},
	}

	var output bytes.Buffer
	renderDependencyMatrix(&output, modules, nil, nil, false)

	want := "| Project | lib |\n" +
		"| --- | --- |\n" +
		"| app | ▲ |\n" +
		"0 ahead, 0 with local changes, 0 deps out of date.\n"
	if got := output.String(); got != want {
		t.Fatalf("renderDependencyMatrix() markdown =\n%s\nwant:\n%s", got, want)
	}
}
