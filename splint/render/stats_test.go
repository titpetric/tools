package render_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/render"
)

// tables is a linter that measured something and found nothing, which is a
// shape the interface allows and a renderer has to handle.
func tables() []model.LintReport {
	return []model.LintReport{
		results{name: "first", stats: []model.Statistics{model.NewStatistics(
			[]string{"Package", "Files"},
			[][]string{{"./model", "12"}, {"./render", "5"}},
			model.HeaderText("What the first linter measured."),
			model.FooterText("17 files in 2 packages."),
		)}},
		results{name: "second", stats: []model.Statistics{model.NewStatistics(
			[]string{"Package", "Ratio"},
			[][]string{{"./model", "1.2%"}},
			model.HeaderText("What the second linter measured."),
			model.FooterText("One package measured."),
		)}},
		// A linter that measures nothing contributes no table, rather than an
		// empty one with a heading over it.
		results{name: "third"},
	}
}

// TestMarkdownStats covers the layout the flag promises: a header above each
// table, a footer under it, and one blank line before the next.
func TestMarkdownStats(t *testing.T) {
	var out bytes.Buffer
	if err := render.MarkdownStats(&out, tables()); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	for _, want := range []string{
		"What the first linter measured.",
		// The columns are padded, which is what mdox leaves alone.
		"| Package  | Files |",
		"17 files in 2 packages.",
		"What the second linter measured.",
		"One package measured.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("MarkdownStats() is missing %q:\n%s", want, got)
		}
	}

	// One blank line between the footer of one table and the header of the
	// next, and no more.
	if !strings.Contains(got, "17 files in 2 packages.\n\nWhat the second linter measured.") {
		t.Errorf("MarkdownStats() did not put one blank line between the tables:\n%q", got)
	}
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("MarkdownStats() left more than one blank line:\n%q", got)
	}

	// A linter with nothing to measure is not announced.
	if strings.Contains(got, "third") {
		t.Errorf("MarkdownStats() wrote a table for a linter that measured nothing:\n%s", got)
	}
}

func TestMarkdownStatsIsPadded(t *testing.T) {
	var out bytes.Buffer
	if err := render.MarkdownStats(&out, tables()); err != nil {
		t.Fatal(err)
	}

	// Every row of one table is the same width, which is what mdox leaves
	// alone. The two tables have different columns, so they are checked apart.
	widths := map[string][]int{}
	table := ""
	for _, line := range strings.Split(out.String(), "\n") {
		if !strings.HasPrefix(line, "|") {
			table = ""
			continue
		}
		if table == "" {
			table = line
		}
		widths[table] = append(widths[table], len(line))
	}

	for header, found := range widths {
		for i, width := range found {
			if width != found[0] {
				t.Errorf("row %d of %q is %d wide and the header is %d", i, header, width, found[0])
			}
		}
	}
}

func TestTerminalStats(t *testing.T) {
	var out bytes.Buffer
	if err := render.TerminalStats(&out, tables()); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	if !strings.Contains(got, "╭") || !strings.Contains(got, "╰") {
		t.Errorf("TerminalStats() drew no box:\n%s", got)
	}
	if !strings.Contains(got, "17 files in 2 packages.") {
		t.Errorf("TerminalStats() lost a footer:\n%s", got)
	}
	if strings.Contains(got, "third") {
		t.Errorf("TerminalStats() wrote a table for a linter that measured nothing")
	}
}

func TestStatsOnNothing(t *testing.T) {
	for _, format := range []render.Format{render.FormatMarkdown, render.FormatTerminal} {
		var out bytes.Buffer
		if err := render.Stats(&out, []model.LintReport{results{name: "quiet"}}, format); err != nil {
			t.Fatal(err)
		}
		// A run where no linter measured anything says so, rather than
		// printing a blank that reads as though nothing ran.
		if !strings.Contains(out.String(), "No linter reported any statistics.") {
			t.Errorf("%s: Stats() = %q", format, out.String())
		}
	}
}

func TestStatsPicksMarkdownWhenRedirected(t *testing.T) {
	var out bytes.Buffer
	if err := render.Stats(&out, tables(), render.FormatAuto); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "| Package  | Files |") {
		t.Errorf("Stats() to a buffer did not write markdown:\n%s", out.String())
	}
	if strings.Contains(out.String(), "\033") {
		t.Error("Stats() to a buffer wrote escape codes")
	}
}
