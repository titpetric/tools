package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/report"
)

// markdownTable renders a table with every column padded to its widest cell.
func markdownTable(header []string, rows [][]string) string {
	widths := make([]int, len(header))
	for i, cell := range header {
		widths[i] = len(cell)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				widths[i] = max(widths[i], len(cell))
			}
		}
	}

	var out strings.Builder
	out.WriteString(markdownRow(header, widths))

	// The rule mdox writes has no space beside the dashes, and spans the
	// padding on either side of the cell.
	rule := make([]string, len(widths))
	for i, width := range widths {
		rule[i] = strings.Repeat("-", width+2)
	}
	out.WriteString("|" + strings.Join(rule, "|") + "|\n")

	for _, cells := range rows {
		out.WriteString(markdownRow(cells, widths))
	}

	return out.String()
}

// markdownRow writes one line with every cell padded to its column.
func markdownRow(cells []string, widths []int) string {
	var out strings.Builder
	out.WriteString("|")

	for i, width := range widths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		out.WriteString(" " + cell + strings.Repeat(" ", width-len(cell)) + " |")
	}

	out.WriteString("\n")
	return out.String()
}

// escape keeps a cell from breaking the table it is in: a pipe in a message
// would open a column that is not there.
func escape(cell string) string {
	cell = strings.ReplaceAll(cell, "|", "\\|")
	return strings.ReplaceAll(cell, "\n", " ")
}

// summary states what ran and what it found, in one line above the table.
func summary(report *report.Report) string {
	counts := report.Counts()
	parts := make([]string, 0, len(report.Linters))
	for _, linter := range report.Linters {
		parts = append(parts, fmt.Sprintf("%s %d", linter, counts[linter]))
	}
	return fmt.Sprintf("%s from %s: %s.",
		plural(report.Len(), "issue", "issues"),
		plural(len(report.Linters), "linter", "linters"),
		strings.Join(parts, ", "))
}

// empty is what a clean run says, which names the linters that found nothing
// rather than printing a blank.
func empty(report *report.Report) string {
	if len(report.Linters) == 0 {
		return "No linters ran."
	}
	return fmt.Sprintf("No issues. %s found nothing.", strings.Join(report.Linters, ", "))
}

// plural writes a count with the word for it.
func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// severityName is the level as a word, which is what slog calls it.
func severityName(severity model.Severity) string {
	return severity.String()
}

// MarkdownStats writes the statistics of every linter as padded markdown
// tables, one per linter, a blank line apart.
//
// The header prints above the table and the footer below it, which is where a
// linter writes the one line a reader takes away.
func MarkdownStats(w io.Writer, reports []model.LintReport) error {
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
				if _, err := fmt.Fprintf(w, "%s\n\n", stats.Header); err != nil {
					return err
				}
			}

			rows := make([][]string, 0, len(stats.Rows))
			for _, cells := range stats.Rows {
				escaped := make([]string, len(cells))
				for i, cell := range cells {
					escaped[i] = escape(cell)
				}
				rows = append(rows, escaped)
			}
			if _, err := io.WriteString(w, markdownTable(stats.Labels, rows)); err != nil {
				return err
			}

			if stats.Footer != "" {
				if _, err := fmt.Fprintf(w, "\n%s\n", stats.Footer); err != nil {
					return err
				}
			}
		}
	}

	if first {
		_, err := fmt.Fprintln(w, "No linter reported any statistics.")
		return err
	}

	return nil
}
