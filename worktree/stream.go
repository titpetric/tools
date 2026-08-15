package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/titpetric/tools/worktree/components"
)

// streamTable renders a table row by row, as results become available. The
// leading columns are padded to fixed widths, the last column is left open so
// a row can be started before its contents are known.
type streamTable struct {
	w       io.Writer
	numCols int
	widths  []int
	styled  bool
	pending []string
}

// newStreamTable writes the table header and returns the writer for its rows.
// The widths hold the display width of each leading column; the last column is
// unbounded and its width is only used for the header rule.
func newStreamTable(w io.Writer, headers []string, widths []int, styled bool) *streamTable {
	t := &streamTable{w: w, numCols: len(headers), widths: widths, styled: styled}
	if !styled {
		writeMarkdownHeader(w, headers)
		return t
	}
	cells := make([]string, 0, len(headers))
	for _, header := range headers {
		cells = append(cells, components.ColorHeader+header+components.ColorReset)
	}
	writeOpenBorder(w, boxTopLeft, boxTeeDown, widths)
	fmt.Fprintln(w, openRowPrefix(cells[:len(cells)-1], widths)+" "+cells[len(cells)-1])
	writeOpenBorder(w, boxTeeRight, boxCross, widths)
	return t
}

// start writes the leading cells of a row, leaving the line open for the
// results of the work in progress.
func (t *streamTable) start(cells ...string) {
	t.pending = cells
	if t.styled {
		fmt.Fprint(t.w, openRowPrefix(cells, t.widths))
	}
}

// finish writes value, which may span multiple lines, as the last column of
// the row started with start, completing the row.
func (t *streamTable) finish(value string) {
	if !t.styled {
		writeMarkdownRow(t.w, t.numCols, toRow(t.numCols, append(t.pending, value)))
		t.pending = nil
		return
	}
	blanks := make([]string, len(t.pending))
	for i, line := range components.Cell(strings.Split(strings.TrimSpace(value), "\n")) {
		if i > 0 {
			fmt.Fprint(t.w, openRowPrefix(blanks, t.widths))
		}
		fmt.Fprintln(t.w, " "+line)
	}
	t.pending = nil
}

// close writes the closing border of the table.
func (t *streamTable) close() {
	if t.styled {
		writeOpenBorder(t.w, boxBottomLeft, boxTeeUp, t.widths)
	}
}

// openRowPrefix renders the padded leading cells of a row, up to and including
// the border in front of the open last column.
func openRowPrefix(cells []string, widths []int) string {
	border := components.ColorSeparator + boxVertical + components.ColorReset
	out := border
	for i, cell := range cells {
		pad := max(widths[i]-ansi.StringWidth(cell), 0)
		out += " " + cell + strings.Repeat(" ", pad) + " " + border
	}
	return out
}

// writeOpenBorder writes a horizontal rule for a table without a right border.
func writeOpenBorder(w io.Writer, left, mid string, widths []int) {
	segs := make([]string, 0, len(widths))
	for _, width := range widths {
		segs = append(segs, strings.Repeat(boxHorizontal, width+2))
	}
	fmt.Fprintln(w, components.ColorSeparator+left+strings.Join(segs, mid)+components.ColorReset)
}
