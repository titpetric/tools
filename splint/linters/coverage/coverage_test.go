package coverage_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/titpetric/tools/splint/linters/coverage"
	"github.com/titpetric/tools/splint/model"
)

// source is the package under test in every case below.
var source = model.Package{Package: "x", ImportPath: "example.com/x", Path: "./x"}

// document builds the two definitions a package arrives as: the package
// itself, and the test package beside it. The parsers report a test package
// separately, under its own name and its own import path, so a linter that
// expected the tests to be in the definition the symbols are in would find
// none.
func document(def *model.Definition, tests ...string) *model.DocumentRoot {
	root := &model.DocumentRoot{Packages: model.DefinitionList{def}}
	if len(tests) == 0 {
		return root
	}

	testDef := &model.Definition{
		Package: model.Package{
			Package:     def.Package.Package + "_test",
			ImportPath:  def.Package.ImportPath + "_test",
			Path:        def.Package.Path,
			TestPackage: true,
		},
		Files: model.FileList{{Name: "x_test.go", Test: true}},
	}
	for _, name := range tests {
		testDef.Funcs = append(testDef.Funcs, &model.Declaration{
			Kind: model.FuncKind,
			Name: name,
			File: "x_test.go",
			Line: 1,
		})
	}

	root.Packages = append(root.Packages, testDef)
	return root
}

// definition wraps declarations into the package the linter reads them from.
func definition(pkg model.Package, types, funcs model.DeclarationList) *model.Definition {
	return &model.Definition{
		Package: pkg,
		Files:   model.FileList{{Name: "x.go"}},
		Types:   types,
		Funcs:   funcs,
	}
}

// lint runs the linter and returns the issues it found.
func lint(t *testing.T, root *model.DocumentRoot) []model.Issue {
	t.Helper()

	report, err := coverage.New().Lint(context.Background(), root)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if report.Linter() != coverage.Name {
		t.Errorf("Linter() = %q, want %q", report.Linter(), coverage.Name)
	}

	issues := model.Issues(report)
	if len(issues) != report.Len() {
		t.Errorf("Len() = %d, walked %d", report.Len(), len(issues))
	}
	return issues
}

func TestLinter_Name(t *testing.T) {
	if got := coverage.New().Name(); got != coverage.Name {
		t.Errorf("Name() = %q, want %q", got, coverage.Name)
	}
}

// TestLinter_Lint covers the names a test may carry and still count as the test
// of a symbol, and the symbols that are nobody's to test.
func TestLinter_Lint(t *testing.T) {
	open := &model.Declaration{Kind: model.FuncKind, Name: "Open", File: "x.go", Line: 3}
	closeMethod := &model.Declaration{Kind: model.FuncKind, Name: "Close", Receiver: "*Client", File: "x.go", Line: 7}
	constructor := &model.Declaration{Kind: model.FuncKind, Name: "NewClient", File: "x.go", Line: 5, Returns: []string{"*Client"}}

	tests := []struct {
		title string
		decl  *model.Declaration
		named []string
		want  string
	}{
		{"the test is named for the function", open, []string{"TestOpen"}, ""},
		{"a benchmark names it as well as a test", open, []string{"BenchmarkOpen"}, ""},
		{"the test says more after an underscore", open, []string{"TestOpen_missingFile"}, ""},
		{"no test names it", open, nil, "TestOpen"},
		{"another symbol's test does not name it", open, []string{"TestClose"}, "TestOpen"},
		{"the method under the underscore form", closeMethod, []string{"TestClient_Close"}, ""},
		{"the method under the run together form", closeMethod, []string{"TestClientClose"}, ""},
		{"the test of the type is not the test of its method", closeMethod, []string{"TestClient"}, "TestClient_Close"},
		{"the constructor under the test of what it returns", constructor, []string{"TestClient"}, ""},
		{"the constructor under its own name", constructor, []string{"TestNewClient"}, ""},
		{"the constructor named by nothing", constructor, nil, "TestNewClient"},
	}

	for _, test := range tests {
		root := document(definition(source, nil, model.DeclarationList{test.decl}), test.named...)
		issues := lint(t, root)

		switch {
		case test.want == "":
			if len(issues) != 0 {
				t.Errorf("%s: reported %q", test.title, issues[0].Message)
			}
		case len(issues) != 1:
			t.Errorf("%s: reported %d issues, want 1", test.title, len(issues))
		default:
			if issues[0].Rule != coverage.RuleUncovered {
				t.Errorf("%s: rule = %q, want %q", test.title, issues[0].Rule, coverage.RuleUncovered)
			}
			if issues[0].Symbol != test.decl.Symbol() {
				t.Errorf("%s: symbol = %q, want %q", test.title, issues[0].Symbol, test.decl.Symbol())
			}
			if want := "exported symbol has no test named " + test.want; issues[0].Message != want {
				t.Errorf("%s: message = %q, want %q", test.title, issues[0].Message, want)
			}
			if issues[0].Position.Ref() != "x/x.go:"+strconv.Itoa(test.decl.Line) {
				t.Errorf("%s: position = %q", test.title, issues[0].Position.Ref())
			}
		}
	}
}

// TestLinter_Lint_skips covers the symbols a test is not expected for. A
// command exports nothing anyone reaches, a test package is not a surface
// anyone tests, an unexported symbol is the package's own business, and a
// generated file is nobody's to write a test for.
func TestLinter_Lint_skips(t *testing.T) {
	untested := model.DeclarationList{{Kind: model.FuncKind, Name: "Open", File: "x.go", Line: 3}}

	for _, pkg := range []model.Package{
		{Package: "main", ImportPath: "example.com/cmd/x", Path: "./cmd/x"},
		{Package: "x_test", ImportPath: "example.com/x_test", Path: "./x", TestPackage: true},
	} {
		if issues := lint(t, document(definition(pkg, nil, untested))); len(issues) != 0 {
			t.Errorf("%s: reported %d issues", pkg.Package, len(issues))
		}
	}

	unexported := model.DeclarationList{{Kind: model.FuncKind, Name: "open", File: "x.go", Line: 3}}
	if issues := lint(t, document(definition(source, nil, unexported))); len(issues) != 0 {
		t.Errorf("reported %d issues on an unexported symbol", len(issues))
	}

	inTestFile := model.DeclarationList{{Kind: model.FuncKind, Name: "Fixture", File: "x_test.go", Line: 3}}
	if issues := lint(t, document(definition(source, nil, inTestFile))); len(issues) != 0 {
		t.Errorf("reported %d issues on a symbol declared by a test", len(issues))
	}

	generated := definition(source, nil, untested)
	generated.Files = model.FileList{{Name: "x.go", Generated: true}}
	if issues := lint(t, document(generated)); len(issues) != 0 {
		t.Errorf("reported %d issues on a generated file", len(issues))
	}
}

// TestResults_Statistics reads the table back, since the numbers a linter
// reports are as much its output as the findings are.
func TestResults_Statistics(t *testing.T) {
	def := definition(source, model.DeclarationList{
		{Kind: model.TypeKind, Name: "Client", File: "x.go", Line: 3},
	}, model.DeclarationList{
		{Kind: model.FuncKind, Name: "NewClient", File: "x.go", Line: 5, Returns: []string{"*Client"}},
		{Kind: model.FuncKind, Name: "Open", File: "x.go", Line: 9},
	})

	report, err := coverage.New().Lint(context.Background(), document(def, "TestClient"))
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	metrics := report.Metrics()
	metric, known := metrics.Packages[source.ImportPath].(coverage.Metric)
	if !known {
		t.Fatalf("Packages[%q] = %#v, want a Metric", source.ImportPath, metrics.Packages[source.ImportPath])
	}

	// TestClient names the type and the constructor that returns it, and
	// nothing names Open.
	want := coverage.Metric{Exported: 3, Covered: 2, Constructors: 1, Uncovered: 1}
	if metric != want {
		t.Errorf("Metric = %+v, want %+v", metric, want)
	}

	stats := report.Statistics()
	if len(stats) != 1 {
		t.Fatalf("Statistics() returned %d tables, want 1", len(stats))
	}
	if len(stats[0].Labels) != 5 {
		t.Errorf("Labels = %v, want five columns", stats[0].Labels)
	}
	if len(stats[0].Rows) != 1 {
		t.Fatalf("Rows = %v, want one row", stats[0].Rows)
	}

	row := []string{source.ImportPath, "3", "2", "66.7%", "1"}
	for i, cell := range row {
		if stats[0].Rows[0][i] != cell {
			t.Errorf("Rows[0][%d] = %q, want %q", i, stats[0].Rows[0][i], cell)
		}
	}
	if stats[0].Header == "" || stats[0].Footer == "" {
		t.Errorf("table reads without a header or a footer: %q, %q", stats[0].Header, stats[0].Footer)
	}
}
