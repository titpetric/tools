package render

import (
	"fmt"
	"io"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/report"
)

// Terminal writes the issues of a report, two lines each, a blank line apart.
//
// A finding is two lines rather than a row of a table: a message is a sentence
// and a position is a path, and a table of both is a table as wide as the
// terminal with one column of it worth reading. What a reader scans is the
// first line, and what tells them what to do is the second.
//
//	WARN frontend/view/page.go:42 (godoc/missing)
//	Page - exported symbol lacks a godoc comment
//
// Nothing is drawn around them. A box is as wide as the longest line in it, so
// one long message widens the frame past the terminal and every finding after
// it wraps; the blank line between findings separates them for nothing.
func Terminal(w io.Writer, found *report.Report) error {
	if found.Len() == 0 {
		_, err := fmt.Fprintln(w, paint(empty(found), colorGrey))
		return err
	}

	if _, err := fmt.Fprintln(w, paint(summary(found), colorGrey)); err != nil {
		return err
	}

	for _, issue := range found.Issues {
		if _, err := fmt.Fprintf(w, "\n%s\n%s\n", where(issue), what(issue)); err != nil {
			return err
		}
	}

	return nil
}

// where is the first line of a finding: the level it is at, the file it is in,
// and the rule that reported it.
func where(issue model.Issue) string {
	line := paint(severityName(issue.Severity), severityColor(issue.Severity)) +
		" " + paint(issue.Position.Ref(), colorTeal)

	if rule := issue.RuleName(); rule != "" {
		line += " " + paint("("+rule+")", colorGrey)
	}
	return line
}

// what is the second line: the symbol the finding is about, and what is wrong
// with it. A finding about a file rather than a symbol is the message alone.
func what(issue model.Issue) string {
	if issue.Symbol == "" {
		return issue.Message
	}
	return paint(issue.Symbol, colorSymbol) + " - " + issue.Message
}

// TerminalStats draws the statistics of every linter, one table each, a blank
// line apart.
func TerminalStats(w io.Writer, reports []model.LintReport) error {
	first := true

	for _, one := range reports {
		for _, stats := range statisticsOf(one) {
			if !first {
				if _, err := fmt.Fprintln(w); err != nil {
					return err
				}
			}
			first = false

			if stats.Header != "" {
				if _, err := fmt.Fprintln(w, paint(stats.Header, colorSection)); err != nil {
					return err
				}
			}
			table(w, stats.Labels, stats.Rows, nil)
			if stats.Footer != "" {
				if _, err := fmt.Fprintln(w, paint(stats.Footer, colorGrey)); err != nil {
					return err
				}
			}
		}
	}

	if first {
		_, err := fmt.Fprintln(w, paint("No linter reported any statistics.", colorGrey))
		return err
	}

	return nil
}

// severityColor is the colour a level is written in, which is what a reader
// scans for.
func severityColor(severity model.Severity) string {
	switch {
	case severity >= model.SeverityError:
		return colorRed
	case severity >= model.SeverityWarn:
		return colorAmber
	case severity >= model.SeverityInfo:
		return colorTeal
	}
	return colorGrey
}

// statisticsOf is what one linter reported, and nothing for a linter that
// measures nothing.
func statisticsOf(one model.LintReport) []model.Statistics {
	if one == nil {
		return nil
	}

	var out []model.Statistics
	for _, stats := range one.Statistics() {
		if !stats.Empty() {
			out = append(out, stats)
		}
	}
	return out
}
