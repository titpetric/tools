package prose_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/linters/prose"
	"github.com/titpetric/tools/splint/model"
)

func baselineTree(t *testing.T, comments ...string) string {
	t.Helper()

	dir := t.TempDir()
	body := &strings.Builder{}
	body.WriteString("package b\n")
	for i, comment := range comments {
		body.WriteString("\n// " + comment + "\nfunc F")
		body.WriteByte(byte('a' + i))
		body.WriteString("() {}\n")
	}

	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte(body.String()), 0o644); err != nil {
		t.Fatalf("writing the baseline: %v", err)
	}
	return dir
}

func lint(t *testing.T, root *model.DocumentRoot, baseline string, opts prose.Options) []model.Issue {
	t.Helper()

	linter := prose.New()
	opts.Baseline = baseline
	linter.Configure(opts)

	report, err := linter.Lint(context.Background(), root)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if report.Linter() != prose.Name {
		t.Errorf("Linter() = %q, want %q", report.Linter(), prose.Name)
	}

	issues := model.Issues(report)
	if len(issues) != report.Len() {
		t.Errorf("Len() = %d, walked %d", report.Len(), len(issues))
	}
	return issues
}

func document(docs ...string) *model.DocumentRoot {
	def := &model.Definition{Package: model.Package{Package: "x", Path: "./x"}}
	for i, doc := range docs {
		def.Funcs = append(def.Funcs, &model.Declaration{
			File: "x/a.go",
			Line: i + 1,
			Name: "F",
			Doc:  doc,
		})
	}
	return &model.DocumentRoot{Packages: model.DefinitionList{def}}
}

func TestLinterName(t *testing.T) {
	if got := prose.New().Name(); got != prose.Name {
		t.Errorf("Name() = %q, want %q", got, prose.Name)
	}
}

func TestLinterReportsAShapeTheBaselineAvoids(t *testing.T) {
	baseline := baselineTree(t,
		"Read reads the frame and answers the value it held.",
		"Write puts the value in the frame.",
		"Close releases the frame.",
	)

	root := document(
		"A holds the frame beside the pool.",
		"B puts the value beside the frame.",
		"C reads the name beside the value.",
		"D writes the box beside the cell.",
	)

	issues := lint(t, root, baseline, prose.Options{Size: 1, Count: 3, Ratio: 1, Score: 1})

	var named bool
	for _, issue := range issues {
		if issue.Rule == prose.RuleShape && strings.Contains(issue.Message, "beside") {
			named = true
			if issue.Position.File == "" {
				t.Errorf("the finding names no file: %+v", issue)
			}
		}
	}
	if !named {
		t.Errorf("no finding named the word the baseline never writes: %+v", issues)
	}
}

func TestLinterIsQuietOnProseLikeItsBaseline(t *testing.T) {
	comments := []string{
		"Read reads the frame and answers the value it held.",
		"Write puts the value in the frame.",
		"Close releases the frame.",
	}

	issues := lint(t, document(comments...), baselineTree(t, comments...),
		prose.Options{Size: 2, Count: 1, Ratio: 10, Score: 15.13})

	if len(issues) != 0 {
		t.Errorf("a corpus measured against itself reported %d findings: %+v", len(issues), issues)
	}
}

func TestLinterReportsADensity(t *testing.T) {
	baseline := baselineTree(t, "Read reads the frame.", "Write puts it back.", "Close lets it go.")

	long := "A materialises the variable cells before the bridge tier dispatches, " +
		"which means the shape table has to be consulted twice for every single " +
		"invocation that crosses the reflect boundary, because the executor holds " +
		"no record of the kind it bound."

	issues := lint(t, document(long), baseline, prose.Options{Size: 4, Count: 99, Ratio: 99, Score: 9999})

	var found []string
	for _, issue := range issues {
		if issue.Rule == prose.RuleDensity {
			found = append(found, issue.Message)
		}
	}
	if len(found) == 0 {
		t.Fatalf("no density finding for a sentence of forty words: %+v", issues)
	}
	for _, message := range found {
		if !strings.Contains(message, "against the baseline's") {
			t.Errorf("the message names no baseline to judge it against: %q", message)
		}
	}
}

func TestLinterIsQuietOnATreeWithNoProse(t *testing.T) {
	issues := lint(t, &model.DocumentRoot{}, baselineTree(t, "Read reads the frame."), prose.Options{})

	if len(issues) != 0 {
		t.Errorf("a document with no prose reported %d findings: %+v", len(issues), issues)
	}
}

func TestLinterReadsMarkdown(t *testing.T) {
	baseline := baselineTree(t, "Read reads the frame.", "Write puts it back.", "Close lets it go.")

	root := document("A reads the frame.")
	root.Prose = []*model.ProseFile{{
		Path: "docs/one.md",
		Blocks: []model.ProseBlock{
			{Line: 3, Text: "The box beside the pool holds a value beside the cell."},
			{Line: 9, Text: "A name beside the frame reads beside the box."},
		},
	}}

	issues := lint(t, root, baseline, prose.Options{Size: 1, Count: 3, Ratio: 1, Score: 1})

	var named bool
	for _, issue := range issues {
		if issue.Rule == prose.RuleShape && strings.Contains(issue.Message, "beside") {
			named = true
		}
	}
	if !named {
		t.Errorf("the markdown reached no finding: %+v", issues)
	}
}

func TestConfigureKeepsWhatItIsNotGiven(t *testing.T) {
	comments := []string{"Read reads the frame.", "Write puts it back."}

	linter := prose.New()
	linter.Configure(prose.Options{Baseline: baselineTree(t, comments...)})

	report, err := linter.Lint(context.Background(), document(comments...))
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	if report.Len() != 0 {
		t.Errorf("Configure cleared the floors: %d findings against an identical corpus", report.Len())
	}
}

func TestStatisticsSayWhatTheCapDropped(t *testing.T) {
	baseline := baselineTree(t,
		"Read reads the frame and answers the value it held.",
		"Write puts the value in the frame.",
	)

	root := document(
		"A holds the frame beside the pool, and the box beside it stands under a name.",
		"B puts the value beside the frame, and the cell beside it stands under a name.",
		"C reads the name beside the value, and the box beside it stands under a name.",
	)

	linter := prose.New()
	linter.Configure(prose.Options{Baseline: baseline, Size: 2, Count: 2, Ratio: 1, Score: 1, Top: 1})

	report, err := linter.Lint(context.Background(), root)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	stats := report.Statistics()
	if len(stats) == 0 {
		t.Fatal("Statistics() answered no table")
	}
	if !strings.Contains(stats[0].Footer, "did not fit the cap") {
		t.Errorf("the footer hides the cap: %q", stats[0].Footer)
	}
}
