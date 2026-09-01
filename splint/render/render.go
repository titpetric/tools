// Package render writes what the linters found and what they measured.
//
// One issue always says the same things: where it is, which rule reported it,
// which symbol it is about, and what is wrong. What changes is the frame
// around them. A terminal gets a box per finding, two lines in it; a redirect
// gets a markdown table; a CI log gets one line per issue, in the shape a
// compiler writes and GitHub Actions resolves against a checkout.
package render

import (
	"io"
	"os"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/report"
)

// Format names a rendering. An empty format is chosen by the destination.
type Format string

const (
	FormatAuto     Format = "auto"
	FormatTerminal Format = "terminal"
	FormatMarkdown Format = "markdown"
	FormatGitHub   Format = "github"
)

// Issues writes what the linters found.
func Issues(w io.Writer, found *report.Report, format Format) error {
	switch resolve(w, format) {
	case FormatTerminal:
		return Terminal(w, found)
	case FormatGitHub:
		return GitHub(w, found)
	}
	return Markdown(w, found)
}

// Stats writes what the linters measured, one table per linter, a blank line
// apart.
func Stats(w io.Writer, reports []model.LintReport, format Format) error {
	switch resolve(w, format) {
	case FormatTerminal:
		return TerminalStats(w, reports)
	case FormatGitHub:
		// A CI log reads lines, and a table is not one. The markdown table is
		// what a log holds that a reader can still take something from.
		return MarkdownStats(w, reports)
	}
	return MarkdownStats(w, reports)
}

// resolve settles which rendering to use, which for auto is decided by where
// the output is going.
func resolve(w io.Writer, format Format) Format {
	switch format {
	case FormatTerminal, FormatMarkdown, FormatGitHub:
		return format
	}

	if IsTerminal(w) {
		return FormatTerminal
	}
	return FormatMarkdown
}

// IsTerminal reports whether a writer is a terminal, which is what decides
// between the two renderings. A dumb terminal is not one: it is what a pager
// or an editor sets when it wants plain text.
func IsTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok || os.Getenv("TERM") == "dumb" {
		return false
	}

	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
