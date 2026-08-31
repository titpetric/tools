package render_test

import (
	"bytes"
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

// TestLine covers the shape GitHub Actions resolves against a checkout.
func TestLine(t *testing.T) {
	issue := model.Issue{
		Linter: "godoc", Rule: "missing",
		Position: model.Position{File: "model/trace.go", Line: 7},
		Message:  "exported symbol lacks a godoc comment",
	}

	want := "model/trace.go:7: godoc/missing: exported symbol lacks a godoc comment"
	if got := report.Line(issue); got != want {
		t.Errorf("Line() = %q, want %q", got, want)
	}

	// A linter with one rule names itself and nothing more.
	issue.Rule = ""
	if got := report.Line(issue); !strings.HasPrefix(got, "model/trace.go:7: godoc: ") {
		t.Errorf("Line() = %q", got)
	}
}

func TestGitHub(t *testing.T) {
	var out bytes.Buffer
	if err := render.GitHub(&out, sample()); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("wrote %d lines, want one per issue", len(lines))
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "/") {
			t.Errorf("line opens on a separator, which no checkout resolves: %q", line)
		}
	}
}

// TestMarkdownIsPadded covers the requirement that the table is mdox clean: a
// document holding it is left alone by "mdox fmt", so every column is padded
// to its width.
func TestMarkdownIsPadded(t *testing.T) {
	var out bytes.Buffer
	if err := render.Markdown(&out, sample()); err != nil {
		t.Fatal(err)
	}

	text := out.String()
	if !strings.Contains(text, "| Position") {
		t.Fatalf("no table was written:\n%s", text)
	}

	var widths []int
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		widths = append(widths, len(line))
	}
	if len(widths) < 4 {
		t.Fatalf("the table has %d rows:\n%s", len(widths), text)
	}
	for i, width := range widths {
		if width != widths[0] {
			t.Errorf("row %d is %d wide and the header is %d, so the table is not padded:\n%s",
				i, width, widths[0], text)
		}
	}
}

func TestMarkdownOnACleanRun(t *testing.T) {
	var out bytes.Buffer
	if err := render.Markdown(&out, report.New(results{name: "godoc"})); err != nil {
		t.Fatal(err)
	}

	// A clean run says which linters found nothing rather than printing a
	// blank, which reads as though nothing ran.
	if got := out.String(); !strings.Contains(got, "godoc") || !strings.Contains(got, "No issues") {
		t.Errorf("Markdown() on a clean run = %q", got)
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

	// Redirected output is what a markdown table is for, so Write picks it.
	var out bytes.Buffer
	if err := render.Issues(&out, sample(), render.FormatAuto); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "| Position") {
		t.Errorf("Write() to a buffer did not write markdown:\n%s", out.String())
	}
	if strings.Contains(out.String(), "\033") {
		t.Error("Write() to a buffer wrote escape codes")
	}
}

func TestTerminalDraws(t *testing.T) {
	var out bytes.Buffer
	if err := render.Terminal(&out, sample()); err != nil {
		t.Fatal(err)
	}

	text := out.String()
	// The columns are the same as the markdown table's, so the two renderings
	// are one report and not two.
	for _, header := range []string{"Position", "Severity", "Rule", "Symbol", "Message"} {
		if !strings.Contains(text, header) {
			t.Errorf("the drawn table has no %q column:\n%s", header, text)
		}
	}
	if !strings.Contains(text, "model/trace.go:7") {
		t.Errorf("the drawn table lost a position:\n%s", text)
	}
}
