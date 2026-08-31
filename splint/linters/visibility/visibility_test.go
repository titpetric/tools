package visibility_test

import (
	"context"
	"testing"

	"github.com/titpetric/tools/splint/linters/visibility"
	"github.com/titpetric/tools/splint/model"
)

// pkg names a package the way a document records one.
func pkg(name, path string) model.Package {
	return model.Package{Package: name, ImportPath: "example.com/" + path, Path: "./" + path}
}

// fn is one func declaration occupying a given number of lines.
func fn(name, receiver, file string, lines int) *model.Declaration {
	return &model.Declaration{
		Kind:       model.FuncKind,
		Name:       name,
		Receiver:   receiver,
		File:       file,
		Complexity: &model.Complexity{Lines: lines},
	}
}

// metricOf returns what the report measured for one import path.
func metricOf(t *testing.T, report model.LintReport, path string) visibility.Metric {
	t.Helper()

	value, known := report.Metrics().Packages[path]
	if !known {
		t.Fatalf("no metric for %q, have %v", path, report.Metrics().PackageKeys())
	}
	metric, ok := value.(visibility.Metric)
	if !ok {
		t.Fatalf("metric for %q is %T", path, value)
	}
	return metric
}

// lint runs the linter over a hand built document.
func lint(t *testing.T, root *model.DocumentRoot) model.LintReport {
	t.Helper()

	report, err := visibility.New().Lint(context.Background(), root)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if report.Linter() != visibility.Name {
		t.Errorf("Linter() = %q, want %q", report.Linter(), visibility.Name)
	}
	return report
}

// TestLinter counts one package and checks that the halves fall where the case
// of a name puts them, and that nothing is reported: the linter measures and
// does not judge.
func TestLinter(t *testing.T) {
	root := &model.DocumentRoot{Packages: model.DefinitionList{{
		Package: pkg("x", "x"),
		Files: model.FileList{
			{Name: "x.go", Lines: 100},
			{Name: "x_test.go", Lines: 40, Test: true},
			{Name: "x_gen.go", Lines: 500, Generated: true},
		},
		Types: model.DeclarationList{
			{Kind: model.TypeKind, Name: "Tracer", File: "x.go"},
			{Kind: model.TypeKind, Name: "span", File: "x.go"},
			{Kind: model.TypeKind, Name: "Generated", File: "x_gen.go"},
			{Kind: model.TypeKind, Name: "Helper", File: "x_test.go"},
		},
		Funcs: model.DeclarationList{
			fn("New", "", "x.go", 5),
			fn("Serve", "*Tracer", "x.go", 7),
			fn("serveHTTP", "*Tracer", "x.go", 11),
			fn("helper", "", "x.go", 9),
			fn("generated", "", "x_gen.go", 400),
			fn("TestServe", "", "x_test.go", 30),
		},
	}}}

	report := lint(t, root)

	if report.Len() != 0 {
		t.Errorf("Len() = %d, want 0: the counts are not judged", report.Len())
	}
	for issue := range report.All() {
		t.Errorf("All() yielded %v, want nothing", issue)
	}

	metric := metricOf(t, report, "example.com/x")
	want := visibility.Metric{
		ExportedTypes: 1,
		InternalTypes: 1,
		ExportedFuncs: 2,
		InternalFuncs: 2,
		InternalLines: 20,
		Lines:         100,
	}
	if metric != want {
		t.Errorf("metric = %+v, want %+v", metric, want)
	}
}

// TestLinter_Lint covers the packages the count leaves out, each of which
// would say something about a surface nobody reads.
func TestLinter_Lint(t *testing.T) {
	files := model.FileList{{Name: "x.go", Lines: 10}}
	decls := model.DeclarationList{fn("Do", "", "x.go", 3)}

	tests := []struct {
		title   string
		def     *model.Definition
		counted bool
	}{
		{
			title:   "a package anyone can import",
			def:     &model.Definition{Package: pkg("public", "public"), Files: files, Funcs: decls},
			counted: true,
		},
		{
			title: "a command exports nothing anyone imports",
			def:   &model.Definition{Package: model.Package{Package: "main", ImportPath: "example.com/cmd", Path: "./cmd"}, Files: files, Funcs: decls},
		},
		{
			title: "a test package is not a surface",
			def:   &model.Definition{Package: model.Package{Package: "x_test", ImportPath: "example.com/x", Path: "./x", TestPackage: true}, Files: files, Funcs: decls},
		},
		{
			title: "an internal tree is already scoped by go",
			def:   &model.Definition{Package: pkg("collector", "analyzer/internal/collector"), Files: files, Funcs: decls},
		},
		{
			title: "a package nobody wrote by hand",
			def:   &model.Definition{Package: pkg("gen", "gen"), Files: model.FileList{{Name: "gen.go", Lines: 90, Generated: true}}, Funcs: decls},
		},
	}

	for _, test := range tests {
		report := lint(t, &model.DocumentRoot{Packages: model.DefinitionList{test.def}})
		counted := len(report.Metrics().Packages) == 1
		if counted != test.counted {
			t.Errorf("%s: counted = %v, want %v", test.title, counted, test.counted)
		}
	}
}

// TestLinter_Statistics checks the one table the linter reports, which is the
// whole of what it has to say.
func TestLinter_Statistics(t *testing.T) {
	root := &model.DocumentRoot{Packages: model.DefinitionList{
		{
			Package: pkg("b", "b"),
			Files:   model.FileList{{Name: "b.go", Lines: 50}},
			Funcs:   model.DeclarationList{fn("Do", "", "b.go", 10), fn("step", "", "b.go", 15)},
		},
		{
			Package: pkg("a", "a"),
			Files:   model.FileList{{Name: "a.go", Lines: 20}},
			Types:   model.DeclarationList{{Kind: model.TypeKind, Name: "Thing", File: "a.go"}},
		},
	}}

	stats := lint(t, root).Statistics()
	if len(stats) != 1 {
		t.Fatalf("Statistics() = %d tables, want 1", len(stats))
	}

	table := stats[0]
	if table.Header == "" || table.Footer == "" {
		t.Errorf("table reads header %q and footer %q, want both", table.Header, table.Footer)
	}
	if len(table.Labels) != 4 {
		t.Fatalf("Labels = %v, want four columns", table.Labels)
	}

	want := [][]string{
		{"example.com/a", "1 / 0", "0 / 0", "0.0%"},
		{"example.com/b", "0 / 0", "1 / 1", "30.0%"},
	}
	if len(table.Rows) != len(want) {
		t.Fatalf("Rows = %d, want %d", len(table.Rows), len(want))
	}
	for i, row := range table.Rows {
		for j, cell := range row {
			if cell != want[i][j] {
				t.Errorf("row %d cell %d = %q, want %q", i, j, cell, want[i][j])
			}
		}
	}
}

// TestMetric_Ratio covers the share a package with no code has, which is none
// rather than a division by zero.
func TestMetric_Ratio(t *testing.T) {
	tests := []struct {
		metric visibility.Metric
		want   float64
	}{
		{metric: visibility.Metric{InternalLines: 25, Lines: 100}, want: 25},
		{metric: visibility.Metric{InternalLines: 0, Lines: 100}, want: 0},
		{metric: visibility.Metric{InternalLines: 10, Lines: 0}, want: 0},
	}

	for _, test := range tests {
		if got := test.metric.Ratio(); got != test.want {
			t.Errorf("Ratio() of %+v = %v, want %v", test.metric, got, test.want)
		}
	}
}

// TestMetric_Exported checks the two halves a reader compares.
func TestMetric_Exported(t *testing.T) {
	metric := visibility.Metric{ExportedTypes: 2, InternalTypes: 3, ExportedFuncs: 5, InternalFuncs: 7}
	if got := metric.Exported(); got != 7 {
		t.Errorf("Exported() = %d, want 7", got)
	}
	if got := metric.Internal(); got != 10 {
		t.Errorf("Internal() = %d, want 10", got)
	}
}
