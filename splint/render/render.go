// Package render writes what the linters found and what they measured.
//
// An issue is one line, whoever is reading: the level, the position, the rule
// and the message, in the shape a compiler writes and a log is read back out
// of. A terminal gets the same line with the parts in colour, so what an
// operator watches and what a CI log holds are one rendering and not two.
//
// The measurements are tables, and a table is drawn for a terminal and written
// as markdown for anything else.
package render

import (
	"io"
	"os"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/report"
)

// Issues writes what the linters found, one line each, with a count above
// them.
func Issues(w io.Writer, found *report.Report) error {
	colour := IsTerminal(w)

	if found.Len() == 0 {
		_, err := writeLine(w, empty(found), colorGrey, colour)
		return err
	}

	if _, err := writeLine(w, summary(found), colorGrey, colour); err != nil {
		return err
	}

	for _, issue := range found.Issues {
		if _, err := io.WriteString(w, compose(issue, colour)+"\n"); err != nil {
			return err
		}
	}

	return nil
}

// Stats writes what the linters measured, one table per linter, a blank line
// apart. A terminal gets the table drawn and anything else gets it in
// markdown.
func Stats(w io.Writer, reports []model.LintReport) error {
	if IsTerminal(w) {
		return TerminalStats(w, reports)
	}
	return MarkdownStats(w, reports)
}

// writeLine writes one line of the frame around the issues, painted where it
// is going to a terminal.
func writeLine(w io.Writer, text, color string, colour bool) (int, error) {
	if colour {
		text = paint(text, color)
	}
	return io.WriteString(w, text+"\n")
}

// IsTerminal reports whether a writer is a terminal, which is what decides
// whether the output carries colour. A dumb terminal is not one: it is what a
// pager or an editor sets when it wants plain text.
func IsTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok || os.Getenv("TERM") == "dumb" {
		return false
	}

	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
