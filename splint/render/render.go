package render

import (
	"io"
	"os"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/report"
)

// Issues writes what the linters found, the way the reader of it reads.
//
// A terminal is a person: they get what each linter found, and then the
// findings themselves. Anything else is a program, and a program that is
// GitHub Actions turns a workflow command into an annotation on a review.
func Issues(w io.Writer, found *report.Report) error {
	if !IsTerminal(w) {
		return Annotations(w, found)
	}

	if found.Len() == 0 {
		_, err := writeLine(w, empty(found), colorGrey, true)
		return err
	}

	if _, err := writeLine(w, summary(found), colorGrey, true); err != nil {
		return err
	}
	if err := Summary(w, found); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}

	for _, issue := range found.Issues {
		if _, err := io.WriteString(w, compose(issue, true)+"\n"); err != nil {
			return err
		}
	}

	return nil
}

// Stats writes one table per linter, drawn for a terminal and markdown otherwise.
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

// IsTerminal reports whether w is a terminal that takes colour, which TERM=dumb,
// set by a pager or an editor, is not.
func IsTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok || os.Getenv("TERM") == "dumb" {
		return false
	}

	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
