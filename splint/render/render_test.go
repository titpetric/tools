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

// TestIssuesToAPipe covers what a log holds: a count, then one line per
// finding, and no escape codes in any of it.
func TestIssuesToAPipe(t *testing.T) {
	var out bytes.Buffer
	if err := render.Issues(&out, sample()); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("wrote %d lines, want a count and one per issue:\n%s", len(lines), out.String())
	}
	if !strings.HasPrefix(lines[0], "2 issues from 2 linters") {
		t.Errorf("the first line is %q", lines[0])
	}
	if strings.Contains(out.String(), "\033") {
		t.Error("a pipe was written escape codes")
	}

	for _, want := range []string{
		"ERROR: model/trace.go:7: godoc/format: Trace - godoc should end in punctuation",
		"WARN: frontend/view/page.go:42: godoc/missing: Page - exported symbol lacks a godoc comment",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("no line reads %q:\n%s", want, out.String())
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
