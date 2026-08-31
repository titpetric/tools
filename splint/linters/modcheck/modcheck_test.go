package modcheck_test

import (
	"context"
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/linters/modcheck"
	"github.com/titpetric/tools/splint/model"
)

// lint runs the linter with no proxy, so the coupling is measured and nothing
// is asked of the network.
func lint(t *testing.T, root *model.DocumentRoot) model.LintReport {
	t.Helper()

	report, err := (&modcheck.Linter{}).Lint(context.Background(), root)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	return report
}

// rules is how many findings there are of each rule.
func rules(report model.LintReport) map[string]int {
	out := map[string]int{}
	for issue := range report.All() {
		out[issue.Rule]++
	}
	return out
}

// document is a module requiring four things and importing two of them.
func document() *model.DocumentRoot {
	return &model.DocumentRoot{
		Modules: []*model.Module{{
			Path: "example.com/main",
			Requires: []model.Require{
				{Path: "example.com/deep", Version: "v1.0.0"},
				{Path: "example.com/thin", Version: "v0.2.0"},
				{Path: "example.com/unused", Version: "v0.1.0"},
				{Path: "example.com/hidden", Version: "v3.0.0", Indirect: true},
			},
		}},
		Packages: model.DefinitionList{
			{
				Package: model.Package{Package: "a", ImportPath: "example.com/main/a", Path: "./a"},
				Imports: model.StringSet{
					"a.go": {`"example.com/deep"`, `"example.com/thin"`},
				},
				Funcs: model.DeclarationList{{
					Kind: model.FuncKind, Name: "Do", File: "a.go",
					References: model.StringSet{
						"deep": {"One", "Two", "Three"},
						"thin": {"Only"},
					},
				}},
			},
			{
				Package: model.Package{Package: "b", ImportPath: "example.com/main/b", Path: "./b"},
				Imports: model.StringSet{"b.go": {`"example.com/deep"`}},
				Funcs: model.DeclarationList{{
					Kind: model.FuncKind, Name: "Go", File: "b.go",
					References: model.StringSet{"deep": {"Four"}},
				}},
			},
		},
	}
}

// TestLinter_Lint covers the counts, which are what say whether a dependency
// can be removed: files is the blast radius, packages the spread, and symbols
// how much of it is used.
func TestLinter_Lint(t *testing.T) {
	metrics := lint(t, document()).Metrics()

	deep, ok := metrics.Packages["example.com/deep"].(modcheck.Dependency)
	if !ok {
		t.Fatalf("metrics hold %v", metrics.PackageKeys())
	}
	if deep.Files != 2 || deep.Packages != 2 || deep.Symbols != 4 {
		t.Errorf("deep = %d files, %d pkgs, %d symbols, want 2, 2, 4", deep.Files, deep.Packages, deep.Symbols)
	}
	if deep.Kind() != "direct" || !deep.Shipped() {
		t.Errorf("deep reads %q, shipped %v", deep.Kind(), deep.Shipped())
	}

	unused := metrics.Packages["example.com/unused"].(modcheck.Dependency)
	if unused.Files != 0 || unused.Kind() != "unused" {
		t.Errorf("unused = %d files, %q", unused.Files, unused.Kind())
	}
	if unused.Shipped() {
		t.Error("a dependency nothing imports is not linked by a consumer")
	}
}

func TestLinter_LintRules(t *testing.T) {
	got := rules(lint(t, document()))

	// One import nothing reaches, and one reached from one file through one
	// symbol. The indirect requirement is nobody's choice and is not reported.
	if got[modcheck.RuleUnused] != 1 {
		t.Errorf("unused = %d, want 1: %v", got[modcheck.RuleUnused], got)
	}
	if got[modcheck.RuleThin] != 1 {
		t.Errorf("thin = %d, want 1: %v", got[modcheck.RuleThin], got)
	}
	if got[modcheck.RuleReplace] != 0 || got[modcheck.RuleMajors] != 0 {
		t.Errorf("reported a replace or a major split that is not there: %v", got)
	}
}

// TestLinter_LintReplace covers the one rule that fails a run: what a replace
// builds against is not what the go.mod requires.
func TestLinter_LintReplace(t *testing.T) {
	root := document()
	root.Modules[0].Replaces = []model.Replace{{Path: "example.com/deep", NewPath: "../deep"}}

	report := lint(t, root)
	if got := rules(report)[modcheck.RuleReplace]; got != 1 {
		t.Fatalf("replace = %d, want 1", got)
	}

	for issue := range report.All() {
		if issue.Rule != modcheck.RuleReplace {
			continue
		}
		if issue.Severity != model.SeverityError {
			t.Errorf("a replace reports at %v, want error: a run has to fail on it", issue.Severity)
		}
		// A module is not in a file, so the finding names the go.mod, which is
		// where a reader goes to act on it.
		if issue.Position.Ref() != "go.mod" {
			t.Errorf("position = %q", issue.Position.Ref())
		}
		if !strings.Contains(issue.Message, "../deep") {
			t.Errorf("message does not say what it resolves to: %q", issue.Message)
		}
	}

	deep := report.Metrics().Packages["example.com/deep"].(modcheck.Dependency)
	if deep.Kind() != "replaced" {
		t.Errorf("deep reads %q, want replaced", deep.Kind())
	}
}

func TestLinter_LintMajors(t *testing.T) {
	root := document()
	root.Modules[0].Requires = append(root.Modules[0].Requires,
		model.Require{Path: "example.com/deep/v2", Version: "v2.0.0"})

	if got := rules(lint(t, root))[modcheck.RuleMajors]; got != 1 {
		t.Errorf("majors = %d, want 1", got)
	}
}

// TestLinter_LintTestOnly covers a dependency a consumer never links, which is
// a different thing from one that is used.
func TestLinter_LintTestOnly(t *testing.T) {
	root := &model.DocumentRoot{
		Modules: []*model.Module{{
			Path:     "example.com/main",
			Requires: []model.Require{{Path: "example.com/testing", Version: "v1.0.0"}},
		}},
		Packages: model.DefinitionList{{
			Package: model.Package{Package: "a_test", ImportPath: "example.com/main/a_test", Path: "./a", TestPackage: true},
			Imports: model.StringSet{"a_test.go": {`"example.com/testing"`}},
		}},
	}

	dep := lint(t, root).Metrics().Packages["example.com/testing"].(modcheck.Dependency)
	if dep.Kind() != "test only" {
		t.Errorf("kind = %q, want test only", dep.Kind())
	}
	if dep.Shipped() {
		t.Error("a test only dependency is not linked by a consumer")
	}
}

// TestLinter_LintDriverImport covers the import that reaches no symbol: a
// driver is imported for what it registers, and there is nothing to inline.
func TestLinter_LintDriverImport(t *testing.T) {
	root := &model.DocumentRoot{
		Modules: []*model.Module{{
			Path:     "example.com/main",
			Requires: []model.Require{{Path: "example.com/driver", Version: "v1.0.0"}},
		}},
		Packages: model.DefinitionList{{
			Package: model.Package{Package: "a", ImportPath: "example.com/main/a", Path: "./a"},
			Imports: model.StringSet{"a.go": {`"example.com/driver"`}},
		}},
	}

	if got := rules(lint(t, root))[modcheck.RuleThin]; got != 0 {
		t.Errorf("thin = %d, want 0 for an import that reaches no symbol", got)
	}
}

func TestLinter_Statistics(t *testing.T) {
	stats := lint(t, document()).Statistics()
	if len(stats) != 1 {
		t.Fatalf("Statistics() = %d tables, want 1", len(stats))
	}

	table := stats[0]
	if table.Header == "" || table.Footer == "" {
		t.Errorf("table reads header %q and footer %q, want both", table.Header, table.Footer)
	}
	if len(table.Labels) != 8 || len(table.Rows) != 4 {
		t.Fatalf("Labels = %v, Rows = %d", table.Labels, len(table.Rows))
	}
	// The size column reads as nothing when nobody was asked, rather than as
	// a zero that looks measured.
	for _, row := range table.Rows {
		if row[2] != "-" {
			t.Errorf("size = %q with no proxy, want %q", row[2], "-")
		}
	}
	if !strings.Contains(table.Footer, "4 dependencies") {
		t.Errorf("footer = %q", table.Footer)
	}
}

func TestLinter_Name(t *testing.T) {
	if got := modcheck.New().Name(); got != modcheck.Name {
		t.Errorf("Name() = %q", got)
	}
}
