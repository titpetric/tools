package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestTable(t *testing.T) {
	var out bytes.Buffer
	table(&out, []string{"Package", "Ratio"}, [][]string{
		{"./model", "1.2%"},
		{"./simpleparser", "88.9%"},
	}, nil)

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 6 {
		t.Fatalf("table() wrote %d lines, want a border, a heading, a rule, two rows and a border:\n%s", len(lines), out.String())
	}

	// Every line is the same width once the colour is taken off, which is what
	// makes it a box rather than a ragged edge.
	width := ansi.StringWidth(lines[0])
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != width {
			t.Errorf("line %d is %d wide and the top is %d:\n%s", i, got, width, out.String())
		}
	}

	// The widest cell decides the column, not the heading.
	if !strings.Contains(lines[0], "─────") || !strings.Contains(ansi.Strip(lines[4]), "./simpleparser") {
		t.Errorf("table() = \n%s", out.String())
	}
}

// TestTableMeasuresWhatIsSeen covers a cell that is already painted: the
// escape codes are not width, and a table that counted them would be ragged.
func TestTableMeasuresWhatIsSeen(t *testing.T) {
	var out bytes.Buffer
	table(&out, []string{"Severity"}, [][]string{
		{paint("WARN", colorAmber)},
		{"INFO"},
	}, nil)

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	width := ansi.StringWidth(lines[0])
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != width {
			t.Errorf("line %d is %d wide and the top is %d, so a painted cell was measured with its escape codes", i, got, width)
		}
	}
}

func TestTableColorsAColumn(t *testing.T) {
	var out bytes.Buffer
	table(&out, []string{"A", "B"}, [][]string{{"one", "two"}}, []string{colorTeal, ""})

	got := out.String()
	if !strings.Contains(got, colorTeal+"one"+colorReset) {
		t.Errorf("table() did not paint the first column:\n%q", got)
	}
	if strings.Contains(got, colorTeal+"two") {
		t.Errorf("table() painted a column it was given no colour for:\n%q", got)
	}
}

func TestPaint(t *testing.T) {
	if got := paint("x", colorTeal); got != colorTeal+"x"+colorReset {
		t.Errorf("paint() = %q", got)
	}
	// Nothing to paint, or nothing to paint it with, is left alone.
	if got := paint("x", ""); got != "x" {
		t.Errorf("paint() = %q", got)
	}
	if got := paint("", colorTeal); got != "" {
		t.Errorf("paint() = %q", got)
	}
}
