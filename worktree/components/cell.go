package components

import "github.com/charmbracelet/x/ansi"

// Cell represents a multi-line table cell. Each element is one display line
// (may contain ANSI color codes).
type Cell []string

// Separator is a sentinel value for a horizontal divider line within a cell.
const Separator = "\x00sep"

// Rows represents a table row as a slice of cells.
type Rows []Cell

// Width returns the maximum visual width across all lines of the cell.
func (c Cell) Width() int {
	w := 0
	for _, line := range c {
		if lw := ansi.StringWidth(line); lw > w {
			w = lw
		}
	}
	return w
}

// Height returns the number of lines in the cell (minimum 1 for empty cells).
func (c Cell) Height() int {
	if len(c) == 0 {
		return 1
	}
	return len(c)
}

// Line returns the i-th line of the cell, or "" if out of range.
func (c Cell) Line(i int) string {
	if i < len(c) {
		return c[i]
	}
	return ""
}
