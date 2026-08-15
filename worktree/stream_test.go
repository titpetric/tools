package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/titpetric/tools/worktree/components"
)

func TestStreamTableWritesRowsAsTheyFinish(t *testing.T) {
	var output bytes.Buffer
	table := newStreamTable(&output, []string{"Path", "Update status"}, []int{8, 13}, true)

	table.start("./alpha")
	started := ansi.Strip(output.String())
	if !strings.HasSuffix(started, "│ ./alpha  │") {
		t.Fatalf("start() did not leave the row open for results:\n%s", started)
	}

	table.finish("Already up to date.")
	table.start("./beta")
	table.finish("example.com/lib v1.0.0 → v1.1.0\n+ example.com/new v0.1.0")
	table.close()

	want := "" +
		"╭──────────┬───────────────\n" +
		"│ Path     │ Update status\n" +
		"├──────────┼───────────────\n" +
		"│ ./alpha  │ Already up to date.\n" +
		"│ ./beta   │ example.com/lib v1.0.0 → v1.1.0\n" +
		"│          │ + example.com/new v0.1.0\n" +
		"╰──────────┴───────────────\n"
	if got := ansi.Strip(output.String()); got != want {
		t.Fatalf("streamTable rendered:\n%s\nwant:\n%s", got, want)
	}
}

func TestStreamTableStylesHeader(t *testing.T) {
	var output bytes.Buffer
	table := newStreamTable(&output, []string{"Path", "Update status"}, []int{4, 13}, true)
	table.close()

	got := output.String()
	for _, want := range []string{
		components.ColorHeader + "Path" + components.ColorReset,
		components.ColorHeader + "Update status" + components.ColorReset,
		components.ColorSeparator + boxVertical + components.ColorReset,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("streamTable header missing %q:\n%q", want, got)
		}
	}
}

func TestStreamTableMarkdown(t *testing.T) {
	var output bytes.Buffer
	table := newStreamTable(&output, []string{"Path", "Update status"}, []int{4, 13}, false)
	table.start("./alpha")
	table.finish(components.ColorAmber + "example.com/lib v1.0.0 → v1.1.0" + components.ColorReset + "\nDone | tidy")
	table.close()

	want := "| Path | Update status |\n" +
		"| --- | --- |\n" +
		"| ./alpha | example.com/lib v1.0.0 → v1.1.0<br>Done \\| tidy |\n"
	if got := output.String(); got != want {
		t.Fatalf("streamTable markdown =\n%s\nwant:\n%s", got, want)
	}
}
