package selfcontained_test

import (
	"context"
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/linters/selfcontained"
	"github.com/titpetric/tools/splint/model"
)

// lint runs the linter over a document.
func lint(t *testing.T, root *model.DocumentRoot) model.LintReport {
	t.Helper()

	report, err := selfcontained.New().Lint(context.Background(), root)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	return report
}

// TestLinter_LintReportsNothing covers the shape of the linter: it measures
// and does not judge, so there is a table and no finding.
func TestLinter_LintReportsNothing(t *testing.T) {
	report := lint(t, document())

	if report.Len() != 0 {
		t.Errorf("Len() = %d, want no findings", report.Len())
	}
	for issue := range report.All() {
		t.Errorf("All() yielded %v", issue)
	}
	if len(report.Statistics()) != 2 {
		t.Fatalf("Statistics() = %d tables, want the packages and the files", len(report.Statistics()))
	}
}

// TestLinter_StatisticsByPackage covers the package table: what a package
// declares, how much of it stands on its own, and the share that does not.
func TestLinter_StatisticsByPackage(t *testing.T) {
	table := lint(t, document()).Statistics()[0]

	if len(table.Labels) != 8 || len(table.Rows) != 1 {
		t.Fatalf("Labels = %v, Rows = %d", table.Labels, len(table.Rows))
	}

	// Package, Files, Types, Types(s), Funcs, Funcs(s), Coupling, Tests.
	want := []string{"example.com/scope", "2", "2", "1", "4", "2", "50.0%", "none"}
	for i, cell := range want {
		if table.Rows[0][i] != cell {
			t.Errorf("row = %v, want %v", table.Rows[0], want)
			break
		}
	}
	if !strings.Contains(table.Footer, "6 symbols, 3 reaching another file") {
		t.Errorf("footer = %q", table.Footer)
	}
}

// TestLinter_StatisticsTestsApart covers the test files, which are counted in
// a column of their own: a test reaches what it tests, and counting it with
// the code would report the coupling of every package as higher than it is.
func TestLinter_StatisticsTestsApart(t *testing.T) {
	root := document()
	root.Packages[0].Files = append(root.Packages[0].Files, model.File{Name: "store_test.go", Test: true})
	root.Packages[0].Funcs = append(root.Packages[0].Funcs, &model.Declaration{
		Kind: model.FuncKind, Name: "TestStore", File: "store_test.go",
		Globals: model.StringSet{"Store": nil},
	})

	table := lint(t, root).Statistics()[0]

	// The code coupling is what it was, and the tests read beside it.
	if got := table.Rows[0][6]; got != "50.0%" {
		t.Errorf("coupling = %q, want the code alone", got)
	}
	if got := table.Rows[0][7]; got != "100.0%" {
		t.Errorf("tests = %q, want 100.0%%", got)
	}
	if got := table.Rows[0][1]; got != "2" {
		t.Errorf("files = %q, want the two that are not tests", got)
	}
}

// TestLinter_StatisticsMostCoupled covers the second table, which is the one
// to act on: the files reaching furthest into the rest of their package.
func TestLinter_StatisticsMostCoupled(t *testing.T) {
	table := lint(t, document()).Statistics()[1]

	if len(table.Rows) != 2 {
		t.Fatalf("Rows = %v", table.Rows)
	}
	// The count comes before the share, so the file with the most coupled
	// symbols heads the table.
	if table.Rows[0][0] != "scope/handler.go" || table.Rows[0][2] != "3" {
		t.Errorf("first row = %v, want handler.go and its three", table.Rows[0])
	}
	if table.Rows[1][0] != "scope/store.go" || table.Rows[1][3] != "0.0%" {
		t.Errorf("second row = %v", table.Rows[1])
	}
}

func TestLinter_Name(t *testing.T) {
	if got := selfcontained.New().Name(); got != selfcontained.Name {
		t.Errorf("Name() = %q", got)
	}
}
