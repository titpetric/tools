package filecheck_test

import (
	"context"
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/linters/filecheck"
	"github.com/titpetric/tools/splint/model"
)

// document wraps a list of files into the document a linter reads.
func document(files ...model.File) *model.DocumentRoot {
	return &model.DocumentRoot{Packages: model.DefinitionList{{
		Package: model.Package{Package: "view", Path: "./frontend/view"},
		Files:   files,
	}}}
}

// lint runs the linter and returns the report, checking the invariants every
// report owes the framework on the way through.
func lint(t *testing.T, root *model.DocumentRoot) model.LintReport {
	t.Helper()

	report, err := filecheck.New().Lint(context.Background(), root)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if report.Linter() != filecheck.Name {
		t.Errorf("Linter() = %q, want %q", report.Linter(), filecheck.Name)
	}
	if issues := model.Issues(report); len(issues) != report.Len() {
		t.Errorf("Len() = %d, walked %d", report.Len(), len(issues))
	}
	return report
}

func TestLinter_Name(t *testing.T) {
	if name := filecheck.New().Name(); name != filecheck.Name {
		t.Errorf("Name() = %q, want %q", name, filecheck.Name)
	}
}

// TestLinter_Lint covers the threshold from both sides, including the file
// sitting exactly on it, which is the one a boundary bug reports.
func TestLinter_Lint(t *testing.T) {
	tests := []struct {
		title  string
		lines  int
		report bool
	}{
		{title: "a short file", lines: 40, report: false},
		{title: "on the threshold", lines: 400, report: false},
		{title: "one line over", lines: 401, report: true},
		{title: "well over", lines: 1200, report: true},
	}

	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			issues := model.Issues(lint(t, document(model.File{Name: "page.go", Lines: test.lines, Size: test.lines * 30})))

			if !test.report {
				if len(issues) != 0 {
					t.Fatalf("reported %d issues on a %d line file", len(issues), test.lines)
				}
				return
			}
			if len(issues) != 1 {
				t.Fatalf("reported %d issues on a %d line file, want 1", len(issues), test.lines)
			}
			if issues[0].Rule != filecheck.RuleLong {
				t.Errorf("rule = %q, want %q", issues[0].Rule, filecheck.RuleLong)
			}
			if issues[0].Symbol != "page.go" {
				t.Errorf("symbol = %q, want the filename", issues[0].Symbol)
			}
			// A file finding names the file and never a line in it.
			if issues[0].Position.Ref() != "frontend/view/page.go" {
				t.Errorf("position = %q, want the file under its package", issues[0].Position.Ref())
			}
		})
	}
}

// TestLinter_Lint_SkipsGeneratedFiles proves a generated file is neither
// reported nor measured, because its length is the generator's and not the
// author's.
func TestLinter_Lint_SkipsGeneratedFiles(t *testing.T) {
	report := lint(t, document(
		model.File{Name: "page_templ.go", Lines: 5000, Size: 200000, Generated: true},
		model.File{Name: "page.go", Lines: 20, Size: 600},
	))

	if report.Len() != 0 {
		t.Errorf("reported %d issues on a generated file", report.Len())
	}
	files := report.Metrics().Files
	if len(files) != 1 {
		t.Fatalf("measured %d files, want the one nobody generated", len(files))
	}
	if _, measured := files["frontend/view/page.go"]; !measured {
		t.Errorf("measured %v, want frontend/view/page.go", report.Metrics().FileKeys())
	}
}

// TestLinter_Lint_CountsAFileOnce guards the case that made count return nil:
// a directory holding an external test package is two definitions naming the
// same path, and a file counted twice is two rows of one histogram bucket.
func TestLinter_Lint_CountsAFileOnce(t *testing.T) {
	pkg := model.Package{Package: "view", Path: "./frontend/view"}
	root := &model.DocumentRoot{Packages: model.DefinitionList{
		{Package: pkg, Files: model.FileList{{Name: "page.go", Lines: 900, Size: 27000}}},
		{
			Package: model.Package{Package: "view_test", Path: "./frontend/view", TestPackage: true},
			Files:   model.FileList{{Name: "page.go", Lines: 900, Size: 27000}},
		},
	}}

	report := lint(t, root)
	if report.Len() != 1 {
		t.Errorf("reported %d issues on one file seen twice, want 1", report.Len())
	}
	if files := report.Metrics().Files; len(files) != 1 {
		t.Errorf("measured %d files, want 1", len(files))
	}
}

// TestLinter_Lint_OnTheRootPackage checks the path of a file in the module
// root, where there is no directory to put in front of the name.
func TestLinter_Lint_OnTheRootPackage(t *testing.T) {
	root := &model.DocumentRoot{Packages: model.DefinitionList{{
		Package: model.Package{Package: "main", Path: "."},
		Files:   model.FileList{{Name: "main.go", Lines: 700, Size: 21000}},
	}}}

	issues := model.Issues(lint(t, root))
	if len(issues) != 1 {
		t.Fatalf("reported %d issues, want 1", len(issues))
	}
	if issues[0].Position.Ref() != "main.go" {
		t.Errorf("position = %q, want main.go", issues[0].Position.Ref())
	}
}

// TestLinter_Lint_Metrics checks that what is measured survives to the metrics
// keyed the way a Position names a file.
func TestLinter_Lint_Metrics(t *testing.T) {
	report := lint(t, document(
		model.File{Name: "page.go", Lines: 600, Size: 18000},
		model.File{Name: "page_test.go", Lines: 80, Size: 2400, Test: true},
	))

	files := report.Metrics().Files
	long, ok := files["frontend/view/page.go"].(filecheck.Metric)
	if !ok {
		t.Fatalf("metrics = %#v, want a Metric per file", files)
	}
	if long.Lines != 600 || long.Size != 18000 || !long.Long || long.Test {
		t.Errorf("metric = %#v, want the long non test file", long)
	}

	short, ok := files["frontend/view/page_test.go"].(filecheck.Metric)
	if !ok {
		t.Fatalf("metrics = %#v, want the test file measured too", files)
	}
	if short.Long || !short.Test {
		t.Errorf("metric = %#v, want a measured test file that was not reported", short)
	}
}

// TestLinter_Lint_Statistics reads the two tables: the histogram has to place
// every file in exactly one bucket, and the longest table has to lead with the
// longest file.
func TestLinter_Lint_Statistics(t *testing.T) {
	report := lint(t, document(
		model.File{Name: "a.go", Lines: 10, Size: 300},
		model.File{Name: "b.go", Lines: 100, Size: 3000},
		model.File{Name: "c.go", Lines: 900, Size: 27000},
	))

	stats := report.Statistics()
	if len(stats) != 2 {
		t.Fatalf("returned %d tables, want the histogram and the longest files", len(stats))
	}

	histogram := stats[0]
	if histogram.Header == "" || histogram.Footer == "" {
		t.Errorf("histogram = %#v, want a header and a one line summary", histogram)
	}
	counted := 0
	for _, row := range histogram.Rows {
		if len(row) != len(histogram.Labels) {
			t.Fatalf("row %v does not fit the labels %v", row, histogram.Labels)
		}
		switch row[1] {
		case "0":
		case "1":
			counted++
		default:
			t.Errorf("bucket %q holds %q files, want one each", row[0], row[1])
		}
	}
	if counted != 3 {
		t.Errorf("histogram placed %d of 3 files", counted)
	}
	if !strings.Contains(histogram.Footer, "3 files") {
		t.Errorf("footer = %q, want the file count in it", histogram.Footer)
	}

	longest := stats[1]
	if len(longest.Rows) != 3 {
		t.Fatalf("longest table has %d rows, want 3", len(longest.Rows))
	}
	if longest.Rows[0][0] != "frontend/view/c.go" {
		t.Errorf("longest table leads with %q, want the longest file", longest.Rows[0][0])
	}
}

// TestLinter_Lint_OnNothing proves an empty document reports and measures
// nothing rather than a table of zeroes.
func TestLinter_Lint_OnNothing(t *testing.T) {
	report := lint(t, &model.DocumentRoot{})

	if report.Len() != 0 {
		t.Errorf("reported %d issues on an empty document", report.Len())
	}
	if !report.Metrics().Empty() {
		t.Errorf("measured %v on an empty document", report.Metrics().FileKeys())
	}
	if stats := report.Statistics(); len(stats) != 0 {
		t.Errorf("returned %d tables on an empty document", len(stats))
	}
}

// TestLinter_Lint_Cancelled proves the linter gives up when the context does,
// which is what makes a run over a large tree interruptible.
func TestLinter_Lint_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := filecheck.New().Lint(ctx, document(model.File{Name: "page.go", Lines: 900})); err == nil {
		t.Error("Lint() on a cancelled context returned no error")
	}
}
