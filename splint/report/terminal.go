package report

import (
	"fmt"
	"io"
	"os"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

	"github.com/titpetric/tools/splint/model"
)

// The palette. Severity is the only thing that carries colour, because
// severity is the data: a reader scanning a long report is looking for the
// errors in it and nothing else.
var (
	stylePosition = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleRule     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleSymbol   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	styleHeader   = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	styleBorder   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleSummary  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// severityStyle is the colour a level is written in.
func severityStyle(severity model.Severity) lipgloss.Style {
	switch {
	case severity >= model.SeverityError:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	case severity >= model.SeverityWarn:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	case severity >= model.SeverityInfo:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	}
	return styleRule
}

// Terminal writes the report as a table drawn for a terminal.
//
// The columns are the same as the markdown table's, and read in the same
// order, so the two renderings are the same report and not two reports.
func Terminal(w io.Writer, report *Report) error {
	if report.Len() == 0 {
		_, err := fmt.Fprintln(w, styleSummary.Render(empty(report)))
		return err
	}

	rows := make([][]string, 0, len(report.Issues))
	severities := make([]model.Severity, 0, len(report.Issues))
	for _, issue := range report.Issues {
		rows = append(rows, []string{
			issue.Position.Ref(),
			severityName(issue.Severity),
			issue.RuleName(),
			issue.Symbol,
			issue.Message,
		})
		severities = append(severities, issue.Severity)
	}

	drawn := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(styleBorder).
		BorderRow(false).
		BorderColumn(true).
		Headers("Position", "Severity", "Rule", "Symbol", "Message").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return styleHeader.Padding(0, 1)
			}
			style := cellStyle(col, severities[row]).Padding(0, 1)
			return style
		}).
		Render()

	if _, err := fmt.Fprintln(w, styleSummary.Render(summary(report))); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, drawn)
	return err
}

// cellStyle is the style of one cell, which is decided by its column and, for
// the severity column, by the level in it.
func cellStyle(col int, severity model.Severity) lipgloss.Style {
	switch col {
	case 0:
		return stylePosition
	case 1:
		return severityStyle(severity)
	case 2:
		return styleRule
	case 3:
		return styleSymbol
	}
	return lipgloss.NewStyle()
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

// Write renders the report the way its destination calls for: drawn for a
// terminal, and as a markdown table for anything else.
func Write(w io.Writer, report *Report) error {
	if IsTerminal(w) {
		return Terminal(w, report)
	}
	return Markdown(w, report)
}
