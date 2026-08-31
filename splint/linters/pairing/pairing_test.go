package pairing_test

import (
	"context"
	"testing"

	"github.com/titpetric/tools/splint/linters/pairing"
	"github.com/titpetric/tools/splint/model"
)

// document wraps a set of definitions into the document a linter reads.
func document(defs ...*model.Definition) *model.DocumentRoot {
	return &model.DocumentRoot{Packages: model.DefinitionList(defs)}
}

// definition is one package of a document, files and nothing else: the linter
// reads the file list and never looks at a declaration.
func definition(pkg model.Package, files ...model.File) *model.Definition {
	return &model.Definition{Package: pkg, Files: model.FileList(files)}
}

// lint runs the linter and returns what it reported.
func lint(t *testing.T, root *model.DocumentRoot) ([]model.Issue, model.LintReport) {
	t.Helper()

	report, err := pairing.New().Lint(context.Background(), root)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if report.Linter() != pairing.Name {
		t.Errorf("Linter() = %q, want %q", report.Linter(), pairing.Name)
	}

	issues := model.Issues(report)
	if len(issues) != report.Len() {
		t.Errorf("Len() = %d, walked %d", report.Len(), len(issues))
	}
	return issues, report
}

// metric returns what the report counted against one import path.
func metric(t *testing.T, report model.LintReport, path string) pairing.Metric {
	t.Helper()

	value, known := report.Metrics().Packages[path]
	if !known {
		t.Fatalf("no metric for %q, have %v", path, report.Metrics().PackageKeys())
	}
	found, ok := value.(pairing.Metric)
	if !ok {
		t.Fatalf("metric for %q is %T", path, value)
	}
	return found
}

// TestLinter_Lint covers the shape the fixture arrives in: a package and its
// tests as two definitions of the same directory, one file paired and one not.
func TestLinter_Lint(t *testing.T) {
	root := document(
		definition(
			model.Package{Package: "paired", ImportPath: "example.com/fixture/paired", Path: "./paired"},
			model.File{Name: "paired.go"},
			model.File{Name: "lonely.go"},
		),
		definition(
			model.Package{Package: "paired_test", ImportPath: "example.com/fixture/paired_test", Path: "./paired", TestPackage: true},
			model.File{Name: "paired_test.go", Test: true},
		),
	)

	issues, report := lint(t, root)
	if len(issues) != 1 {
		t.Fatalf("reported %d issues, want 1: %#v", len(issues), issues)
	}
	if issues[0].Rule != pairing.RuleUnpaired {
		t.Errorf("rule = %q, want %q", issues[0].Rule, pairing.RuleUnpaired)
	}
	if issues[0].Position.Ref() != "paired/lonely.go" {
		t.Errorf("position = %q, want %q", issues[0].Position.Ref(), "paired/lonely.go")
	}
	if issues[0].Message != "lonely.go has no lonely_test.go beside it" {
		t.Errorf("message = %q", issues[0].Message)
	}

	// The two definitions are one package, counted once and under the import
	// path a consumer would name.
	if keys := report.Metrics().PackageKeys(); len(keys) != 1 {
		t.Fatalf("counted %v, want one package", keys)
	}
	want := pairing.Metric{Files: 2, Tests: 1, Paired: 1, StandaloneFiles: 1}
	if got := metric(t, report, "example.com/fixture/paired"); got != want {
		t.Errorf("metric = %+v, want %+v", got, want)
	}
}

// TestLinter_Lint_Generated covers the files nobody wrote, which are neither
// reported nor counted on either side of the pairing.
func TestLinter_Lint_Generated(t *testing.T) {
	root := document(definition(
		model.Package{Package: "x", ImportPath: "example.com/x", Path: "./x"},
		model.File{Name: "zz_generated.go", Generated: true},
		model.File{Name: "x.go"},
		model.File{Name: "x_test.go", Test: true},
	))

	issues, report := lint(t, root)
	if len(issues) != 0 {
		t.Fatalf("reported %d issues, want none: %#v", len(issues), issues)
	}

	want := pairing.Metric{Files: 1, Tests: 1, Paired: 1}
	if got := metric(t, report, "example.com/x"); got != want {
		t.Errorf("metric = %+v, want %+v", got, want)
	}
}

// TestLinter_Lint_StandaloneTest covers a test naming a file that is not
// there, which is counted and has nowhere to be reported.
func TestLinter_Lint_StandaloneTest(t *testing.T) {
	root := document(definition(
		model.Package{Package: "x", ImportPath: "example.com/x", Path: "./x"},
		model.File{Name: "moved_test.go", Test: true},
	))

	issues, report := lint(t, root)
	if len(issues) != 0 {
		t.Fatalf("reported %d issues, want none: %#v", len(issues), issues)
	}

	want := pairing.Metric{Tests: 1, StandaloneTests: 1}
	if got := metric(t, report, "example.com/x"); got != want {
		t.Errorf("metric = %+v, want %+v", got, want)
	}
}

// TestLinter_Lint_Empty covers a document holding nothing, which is a linter
// with nothing to say rather than a linter that failed.
func TestLinter_Lint_Empty(t *testing.T) {
	issues, report := lint(t, document())
	if len(issues) != 0 {
		t.Errorf("reported %d issues on an empty document", len(issues))
	}
	if !report.Metrics().Empty() {
		t.Errorf("counted %v on an empty document", report.Metrics().PackageKeys())
	}
	for _, table := range report.Statistics() {
		if !table.Empty() {
			t.Errorf("tabled %v on an empty document", table.Rows)
		}
	}
}

// TestLinter_Lint_Cancelled covers the context the framework hands in: a run
// that was cancelled reports the cancellation rather than a partial answer.
func TestLinter_Lint_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	root := document(definition(model.Package{Package: "x", Path: "./x"}, model.File{Name: "x.go"}))
	if _, err := pairing.New().Lint(ctx, root); err == nil {
		t.Error("Lint() error = nil on a cancelled context")
	}
}

// TestResults_Statistics covers the table and the one line under it.
func TestResults_Statistics(t *testing.T) {
	root := document(
		definition(
			model.Package{Package: "a", ImportPath: "example.com/a", Path: "./a"},
			model.File{Name: "a.go"},
			model.File{Name: "a_test.go", Test: true},
		),
		definition(
			model.Package{Package: "b", ImportPath: "example.com/b", Path: "./b"},
			model.File{Name: "b.go"},
			model.File{Name: "gone_test.go", Test: true},
		),
	)

	_, report := lint(t, root)
	tables := report.Statistics()
	if len(tables) != 1 {
		t.Fatalf("reported %d tables, want 1", len(tables))
	}

	table := tables[0]
	if len(table.Labels) != 6 {
		t.Errorf("labels = %v", table.Labels)
	}
	if len(table.Rows) != 2 {
		t.Fatalf("rows = %v", table.Rows)
	}
	if table.Rows[0][0] != "example.com/a" || table.Rows[1][0] != "example.com/b" {
		t.Errorf("rows are out of order: %v", table.Rows)
	}
	if got := table.Rows[1]; got[3] != "0" || got[4] != "1" || got[5] != "1" {
		t.Errorf("row = %v, want one standalone file and one standalone test", got)
	}

	want := "1 of 2 files have a test beside them across 2 packages, leaving 1 file and 1 test standing alone."
	if table.Footer != want {
		t.Errorf("footer = %q, want %q", table.Footer, want)
	}
	if table.Header == "" {
		t.Error("header is empty")
	}
}

// TestLinter_Name covers the name the linter is selected by.
func TestLinter_Name(t *testing.T) {
	if got := pairing.New().Name(); got != pairing.Name {
		t.Errorf("Name() = %q, want %q", got, pairing.Name)
	}
}
