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

// TestTerminalDraws covers the shape a finding is read in: one box each, two
// lines, and a blank line between them.
func TestTerminalDraws(t *testing.T) {
	var out bytes.Buffer
	if err := render.Terminal(&out, sample()); err != nil {
		t.Fatal(err)
	}

	text := out.String()
	// Nothing is drawn around a finding: a box is as wide as the longest line
	// in it, and one long message would widen it past the terminal.
	if strings.ContainsAny(text, "╭│╯") {
		t.Errorf("a finding is drawn in a box:\n%s", text)
	}
	if strings.Contains(text, "Position") || strings.Contains(text, "Severity") {
		t.Errorf("the findings carry column headings:\n%s", text)
	}

	// The first line is the level, the file and the rule; the second is the
	// symbol and what is wrong with it.
	plain := strip(text)
	for _, want := range []string{
		"WARN frontend/view/page.go:42 (godoc/missing)",
		"Page - exported symbol lacks a godoc comment",
		"ERROR model/trace.go:7 (godoc/format)",
		"Trace - godoc should end in punctuation",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("no line reads %q:\n%s", want, plain)
		}
	}

	// A blank line between the findings, and none after the last.
	if !strings.Contains(plain, "godoc comment\n\nERROR") {
		t.Errorf("the findings are not a line apart:\n%s", plain)
	}
	if strings.HasSuffix(plain, "\n\n") {
		t.Errorf("the report ends on a blank line:\n%q", plain)
	}
}

// TestTerminalPaints covers what carries colour: the level, the position, the
// rule and the symbol, and not the message.
func TestTerminalPaints(t *testing.T) {
	var out bytes.Buffer
	if err := render.Terminal(&out, sample()); err != nil {
		t.Fatal(err)
	}

	text := out.String()
	for name, want := range map[string]string{
		"the level":   "\033[38;5;214mWARN",
		"an error":    "\033[38;5;167mERROR",
		"the file":    "\033[38;5;72mfrontend/view/page.go:42",
		"the rule":    "\033[38;5;245m(godoc/missing)",
		"the symbol":  "\033[38;5;141mPage",
		"the message": "- exported symbol lacks a godoc comment",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("%s does not read as %q:\n%q", name, want, text)
		}
	}
}

// TestTerminalWithoutASymbol covers a finding about a file rather than about a
// symbol: the second line is the message and nothing in front of it.
func TestTerminalWithoutASymbol(t *testing.T) {
	var out bytes.Buffer
	found := report.New(results{name: "filecheck", issues: []model.Issue{{
		Linter: "filecheck", Rule: "long", Severity: model.SeverityWarn,
		Position: model.Position{File: "frontend/handler.go"},
		Message:  "handler.go runs to 612 lines of code",
	}}})

	if err := render.Terminal(&out, found); err != nil {
		t.Fatal(err)
	}

	plain := strip(out.String())
	if !strings.Contains(plain, "WARN frontend/handler.go (filecheck/long)") {
		t.Errorf("the first line reads wrong:\n%s", plain)
	}
	if !strings.Contains(plain, "\nhandler.go runs to 612 lines of code\n") {
		t.Errorf("the message is not on its own:\n%s", plain)
	}
}

func TestTerminalOnACleanRun(t *testing.T) {
	var out bytes.Buffer
	if err := render.Terminal(&out, report.New(results{name: "godoc"})); err != nil {
		t.Fatal(err)
	}

	plain := strip(out.String())
	if strings.Contains(plain, "\n\n") {
		t.Errorf("a clean run wrote more than the one line:\n%s", plain)
	}
	if !strings.Contains(plain, "godoc") || !strings.Contains(plain, "No issues") {
		t.Errorf("Terminal() on a clean run = %q", plain)
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
