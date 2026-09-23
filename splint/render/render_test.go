package render_test

import (
	"bytes"
	"io"
	"iter"
	"os"
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/render"
	"github.com/titpetric/tools/splint/report"
)

// results is a linter report standing in for a real one, which is all the
// framework ever sees of a linter.
type results struct {
	name   string
	issues []model.Issue
	stats  []model.Statistics
}

func (r results) Linter() string                 { return r.name }
func (r results) Len() int                       { return len(r.issues) }
func (r results) Metrics() model.LintMetrics     { return model.LintMetrics{} }
func (r results) Statistics() []model.Statistics { return r.stats }
func (r results) All() iter.Seq[model.Issue] {
	return func(yield func(model.Issue) bool) {
		for _, issue := range r.issues {
			if !yield(issue) {
				return
			}
		}
	}
}

func sample() *report.Report {
	return report.New(
		results{name: "godoc", issues: []model.Issue{
			{
				Linter: "godoc", Rule: "missing", Severity: model.SeverityWarn,
				Position: model.Position{Package: "view", File: "frontend/view/page.go", Line: 42},
				Symbol:   "Page", Message: "exported symbol lacks a godoc comment",
			},
			{
				Linter: "godoc", Rule: "format", Severity: model.SeverityError,
				Position: model.Position{Package: "model", File: "model/trace.go", Line: 7},
				Symbol:   "Trace", Message: "godoc should end in punctuation",
			},
		}},
		results{name: "imports"},
	)
}

// TestLine covers the one shape an issue is written in: the level, where it
// is, which rule reported it, and what is wrong with the symbol it is about.
func TestLine(t *testing.T) {
	issue := model.Issue{
		Linter: "godoc", Rule: "missing", Severity: model.SeverityWarn,
		Position: model.Position{File: "model/trace.go", Line: 7},
		Symbol:   "Trace",
		Message:  "exported symbol lacks a godoc comment",
	}

	want := "WARN: model/trace.go:7: godoc/missing: Trace - exported symbol lacks a godoc comment"
	if got := render.Line(issue); got != want {
		t.Errorf("Line() = %q, want %q", got, want)
	}

	// A linter with one rule names itself and nothing more, and a finding
	// about a file rather than a symbol is the message after the rule.
	issue.Rule, issue.Symbol = "", ""
	want = "WARN: model/trace.go:7: godoc: exported symbol lacks a godoc comment"
	if got := render.Line(issue); got != want {
		t.Errorf("Line() = %q, want %q", got, want)
	}
}

// TestIssuesToAPipe covers what a program is written: one workflow command per
// finding, which is what GitHub Actions turns into an annotation.
func TestIssuesToAPipe(t *testing.T) {
	var out bytes.Buffer
	if err := render.Issues(&out, sample()); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("wrote %d lines, want one per issue:\n%s", len(lines), out.String())
	}
	if strings.Contains(out.String(), "\033") {
		t.Error("a pipe was written escape codes")
	}

	for _, want := range []string{
		"::error file=model/trace.go,line=7,title=godoc/format::Trace - godoc should end in punctuation",
		"::warning file=frontend/view/page.go,line=42,title=godoc/missing::Page - exported symbol lacks a godoc comment",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("no line reads %q:\n%s", want, out.String())
		}
	}
}

// TestAnnotation covers the command one finding is written as, and what has to
// be escaped in it.
func TestAnnotation(t *testing.T) {
	issue := model.Issue{
		Linter: "filecheck", Rule: "long", Severity: model.SeverityInfo,
		Position: model.Position{File: "./frontend/handler.go", Line: 12, EndLine: 640},
		Message:  "handler.go runs to 612 lines",
	}

	// A level below a warning is a notice, the path never opens on a
	// separator, and a block carries the line it ends on.
	want := "::notice file=frontend/handler.go,line=12,endLine=640,title=filecheck/long::handler.go runs to 612 lines"
	if got := render.Annotation(issue); got != want {
		t.Errorf("Annotation() = %q, want %q", got, want)
	}

	// A message is read to the end of the line, so a line ending in one is
	// written as an escape, and so is the percent that opens one. A property
	// is read to the next comma or colon, so those are written as escapes too.
	issue.Rule = "long,ish:er"
	issue.Position = model.Position{File: "handler.go"}
	issue.Message = "100% of\nit"
	want = "::notice file=handler.go,title=filecheck/long%2Cish%3Aer::100%25 of%0Ait"
	if got := render.Annotation(issue); got != want {
		t.Errorf("Annotation() = %q, want %q", got, want)
	}
}

// TestSummaryOfLinters covers what a terminal reads first: what each linter
// found, at which level, and under which of its rules.
func TestSummaryOfLinters(t *testing.T) {
	var out bytes.Buffer
	if err := render.Summary(&out, sample()); err != nil {
		t.Fatal(err)
	}

	plain := strip(out.String())
	for _, want := range []string{
		"Linter", "Error", "Warn", "Info", "Total", "Rules",
		"godoc", "format 1, missing 1",
		// A linter that ran and found nothing is named, not tabulated.
		"imports found nothing.",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("the summary does not hold %q:\n%s", want, plain)
		}
	}
}

func TestIssuesOnACleanRun(t *testing.T) {
	var out bytes.Buffer
	if err := render.Issues(&out, report.New(results{name: "godoc"})); err != nil {
		t.Fatal(err)
	}

	// A clean run says which linters found nothing rather than printing a
	// blank, which reads as though nothing ran.
	if got := out.String(); !strings.Contains(got, "godoc") || !strings.Contains(got, "No issues") {
		t.Errorf("Issues() on a clean run = %q", got)
	}
}

func TestIsTerminal(t *testing.T) {
	if render.IsTerminal(&bytes.Buffer{}) {
		t.Error("IsTerminal() said a buffer is a terminal")
	}

	file, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if render.IsTerminal(file) {
		t.Error("IsTerminal() said a regular file is a terminal")
	}
}

// strip removes the escape codes, so a test can read what a reader reads.
func strip(text string) string {
	var out strings.Builder

	for i := 0; i < len(text); i++ {
		if text[i] != 0x1b {
			out.WriteByte(text[i])
			continue
		}
		for i < len(text) && text[i] != 'm' {
			i++
		}
	}

	return out.String()
}

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
	for name, write := range map[string]func(io.Writer, []model.LintReport) error{
		"markdown": render.MarkdownStats,
		"terminal": render.TerminalStats,
	} {
		var out bytes.Buffer
		if err := write(&out, []model.LintReport{results{name: "quiet"}}); err != nil {
			t.Fatal(err)
		}
		// A run where no linter measured anything says so, rather than
		// printing a blank that reads as though nothing ran.
		if !strings.Contains(out.String(), "No linter reported any statistics.") {
			t.Errorf("%s: Stats() = %q", name, out.String())
		}
	}
}

func TestStatsPicksMarkdownWhenRedirected(t *testing.T) {
	var out bytes.Buffer
	if err := render.Stats(&out, tables()); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "| Package  | Files |") {
		t.Errorf("Stats() to a buffer did not write markdown:\n%s", out.String())
	}
	if strings.Contains(out.String(), "\033") {
		t.Error("Stats() to a buffer wrote escape codes")
	}
}
