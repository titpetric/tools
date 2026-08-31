package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// The box the tables are drawn in, which is the one worktree draws.
const (
	boxTopLeft     = "╭"
	boxTopRight    = "╮"
	boxBottomLeft  = "╰"
	boxBottomRight = "╯"
	boxHorizontal  = "─"
	boxVertical    = "│"
	boxTeeDown     = "┬"
	boxTeeUp       = "┴"
	boxTeeRight    = "├"
	boxTeeLeft     = "┤"
	boxCross       = "┼"
)

// table draws a table: a top border, the headings, a rule, the rows, and a
// bottom border.
//
// A cell is measured by what it takes on screen rather than by its length, so
// a coloured cell lines up with a plain one: the escape codes are not width.
func table(w io.Writer, labels []string, rows [][]string, colors []string) {
	widths := columnWidths(labels, rows)

	border(w, boxTopLeft, boxTeeDown, boxTopRight, widths)
	headings(w, labels, widths)
	border(w, boxTeeRight, boxCross, boxTeeLeft, widths)
	for _, cells := range rows {
		row(w, cells, widths, colors)
	}
	border(w, boxBottomLeft, boxTeeUp, boxBottomRight, widths)
}

// columnWidths is what each column takes, which is the widest of the heading
// and the cells under it.
func columnWidths(labels []string, rows [][]string) []int {
	widths := make([]int, len(labels))
	for i, label := range labels {
		widths[i] = ansi.StringWidth(label)
	}

	for _, cells := range rows {
		for i, cell := range cells {
			if i < len(widths) {
				widths[i] = max(widths[i], ansi.StringWidth(cell))
			}
		}
	}

	return widths
}

// border draws one horizontal rule of the box.
func border(w io.Writer, left, middle, right string, widths []int) {
	segments := make([]string, 0, len(widths))
	for _, width := range widths {
		segments = append(segments, strings.Repeat(boxHorizontal, width+2))
	}

	fmt.Fprintln(w, paint(left+strings.Join(segments, middle)+right, colorSeparator))
}

// headings draws the column headings.
func headings(w io.Writer, labels []string, widths []int) {
	cells := make([]string, 0, len(labels))
	for i, label := range labels {
		pad := widths[i] - ansi.StringWidth(label)
		cells = append(cells, " "+paint(label, colorHeader)+strings.Repeat(" ", pad)+" ")
	}

	fmt.Fprintln(w, join(cells))
}

// row draws one row, with each cell in the colour its column carries.
func row(w io.Writer, cells []string, widths []int, colors []string) {
	written := make([]string, 0, len(widths))

	for i, width := range widths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		pad := max(0, width-ansi.StringWidth(cell))

		color := ""
		if i < len(colors) {
			color = colors[i]
		}
		written = append(written, " "+paint(cell, color)+strings.Repeat(" ", pad)+" ")
	}

	fmt.Fprintln(w, join(written))
}

// join puts the vertical rules between the cells of a row and on either end.
func join(cells []string) string {
	rule := paint(boxVertical, colorSeparator)
	return rule + strings.Join(cells, rule) + rule
}
