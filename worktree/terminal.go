package main

import (
	"io"
	"os"

	"github.com/charmbracelet/x/term"
)

// supportsANSI reports whether w is a terminal that will read escape codes as
// escape codes, which is what decides between the drawn tables and markdown.
//
// The question is asked of the device rather than of its mode: every character
// device reads as one under os.ModeCharDevice, so a run redirected to /dev/null
// would otherwise be written to in colour.
func supportsANSI(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok || os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(f.Fd())
}

// terminalWidth returns the width of the terminal behind w, or zero when there
// is none to measure.
func terminalWidth(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return 0
	}
	width, _, err := term.GetSize(f.Fd())
	if err != nil {
		return 0
	}
	return width
}

// cellWidth returns the width left for the open last column of a table whose
// leading columns have the given widths. A terminal too narrow to fold into is
// reported as zero, which leaves the cell unfolded.
func cellWidth(terminal int, widths []int) int {
	if terminal <= 0 {
		return 0
	}
	// Each leading column costs its width, a space either side, and the
	// border that follows it; the border opening the row costs one more.
	prefix := 1
	for _, width := range widths[:len(widths)-1] {
		prefix += width + 3
	}
	if left := terminal - prefix - 1; left >= 20 {
		return left
	}
	return 0
}
