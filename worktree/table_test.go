package main

import (
	"bytes"
	"testing"

	"github.com/titpetric/tools/worktree/components"
)

func TestWriteSimpleTableMarkdown(t *testing.T) {
	var output bytes.Buffer
	writeSimpleTable(&output,
		[]string{"Path", "Pull status"},
		[][]string{{"./service", components.ColorGreen + "Updating | files\nDone" + components.ColorReset}},
		false,
	)

	want := "| Path | Pull status |\n" +
		"| --- | --- |\n" +
		"| ./service | Updating \\| files<br>Done |\n"
	if got := output.String(); got != want {
		t.Fatalf("writeSimpleTable() markdown =\n%s\nwant:\n%s", got, want)
	}
}

func TestHeaderWidths(t *testing.T) {
	got := headerWidths([]string{"Path", "Update status"})
	if want := []int{4, 13}; got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("headerWidths() = %v, want %v", got, want)
	}
}
