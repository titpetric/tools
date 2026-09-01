package render

import (
	"fmt"
	"io"

	"github.com/titpetric/tools/splint/model"
)

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
