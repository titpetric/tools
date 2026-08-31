package grouping_test

import (
	"context"
	"testing"

	"github.com/titpetric/tools/splint/linters/grouping"
	"github.com/titpetric/tools/splint/model"
)

// zoo is the package the fixtures below are declared in, so a file named for
// the package is zoo.go and every other name has to be earned.
var zoo = model.Package{Package: "zoo", Path: "./zoo", ImportPath: "example.com/zoo"}

// document wraps lists of declarations into the document a linter reads.
func document(pkg model.Package, types, funcs model.DeclarationList, files model.FileList) *model.DocumentRoot {
	return &model.DocumentRoot{Packages: model.DefinitionList{{
		Package: pkg,
		Files:   files,
		Types:   types,
		Funcs:   funcs,
	}}}
}

// lint runs the linter and returns the issues it found.
func lint(t *testing.T, root *model.DocumentRoot) []model.Issue {
	t.Helper()

	report, err := grouping.New().Lint(context.Background(), root)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if report.Linter() != grouping.Name {
		t.Errorf("Linter() = %q, want %q", report.Linter(), grouping.Name)
	}

	issues := model.Issues(report)
	if len(issues) != report.Len() {
		t.Errorf("Len() = %d, walked %d", report.Len(), len(issues))
	}
	return issues
}

// TestLinterReportsATypeAwayFromItsFile covers the whole of the rule for a
// type: the name is the filename, and the file named for the package holds
// whatever has not been split out of it yet.
func TestLinterReportsATypeAwayFromItsFile(t *testing.T) {
	tests := []struct {
		file   string
		report bool
	}{
		{file: "elephant.go", report: false},
		{file: "zoo.go", report: false},
		{file: "model.go", report: false},
		{file: "animals.go", report: true},
	}

	for _, test := range tests {
		types := model.DeclarationList{
			{Kind: model.TypeKind, Name: "Elephant", File: test.file, Line: 5},
		}

		issues := lint(t, document(zoo, types, nil, nil))
		if !test.report {
			if len(issues) != 0 {
				t.Errorf("%s: reported %q", test.file, issues[0].Message)
			}
			continue
		}

		if len(issues) != 1 {
			t.Fatalf("%s: reported %d issues, want 1", test.file, len(issues))
		}
		if issues[0].Rule != grouping.RuleFilename {
			t.Errorf("%s: rule = %q, want %q", test.file, issues[0].Rule, grouping.RuleFilename)
		}
		if issues[0].Symbol != "Elephant" {
			t.Errorf("%s: symbol = %q", test.file, issues[0].Symbol)
		}
		if issues[0].Position.Ref() != "zoo/animals.go:5" {
			t.Errorf("%s: position = %q", test.file, issues[0].Position.Ref())
		}
		want := `exported type "Elephant" expected in [elephant.go] (total: 4 expected filenames)`
		if issues[0].Message != want {
			t.Errorf("%s: message = %q, want %q", test.file, issues[0].Message, want)
		}
	}
}

// TestLinterReadsAFuncByWhatItBelongsTo covers the two ways a function belongs
// to a type, and the case where it belongs to none.
func TestLinterReadsAFuncByWhatItBelongsTo(t *testing.T) {
	tests := []struct {
		title    string
		decl     *model.Declaration
		symbol   string
		reported bool
	}{
		{
			title:    "a method is filed under its receiver",
			decl:     &model.Declaration{Kind: model.FuncKind, Name: "Close", Receiver: "*Client", File: "server.go", Line: 9},
			symbol:   "Client.Close",
			reported: true,
		},
		{
			title: "and passes in the file named for it",
			decl:  &model.Declaration{Kind: model.FuncKind, Name: "Close", Receiver: "*Client", File: "client_close.go", Line: 9},
		},
		{
			title:    "a constructor is filed under what it returns",
			decl:     &model.Declaration{Kind: model.FuncKind, Name: "NewClient", Returns: []string{"*Client", "error"}, File: "server.go", Line: 9},
			symbol:   "Client.NewClient",
			reported: true,
		},
		{
			title: "and passes in the file named for that",
			decl:  &model.Declaration{Kind: model.FuncKind, Name: "NewClient", Returns: []string{"*Client", "error"}, File: "client.go", Line: 9},
		},
		{
			title: "a function returning nothing of its own belongs nowhere",
			decl:  &model.Declaration{Kind: model.FuncKind, Name: "Serve", File: "server.go", Line: 9},
		},
		{
			title: "and neither does one returning a shape built from a type",
			decl:  &model.Declaration{Kind: model.FuncKind, Name: "All", Returns: []string{"[]Client"}, File: "server.go", Line: 9},
		},
		{
			title: "an unexported receiver is nobody's business",
			decl:  &model.Declaration{Kind: model.FuncKind, Name: "Close", Receiver: "*client", File: "server.go", Line: 9},
		},
	}

	for _, test := range tests {
		issues := lint(t, document(zoo, nil, model.DeclarationList{test.decl}, nil))
		switch {
		case !test.reported && len(issues) > 0:
			t.Errorf("%s: reported %q", test.title, issues[0].Message)
		case test.reported && len(issues) != 1:
			t.Errorf("%s: reported %d issues, want 1", test.title, len(issues))
		case test.reported && issues[0].Symbol != test.symbol:
			t.Errorf("%s: symbol = %q, want %q", test.title, issues[0].Symbol, test.symbol)
		}
	}
}

// TestLinterSkipsWhatItDoesNotJudge covers everything the rule has nothing to
// say about, which is more than what it reports on.
func TestLinterSkipsWhatItDoesNotJudge(t *testing.T) {
	misplaced := func(name, file string) model.DeclarationList {
		return model.DeclarationList{{Kind: model.TypeKind, Name: name, File: file, Line: 5}}
	}

	// A command exports nothing anyone imports, and a test package is not a
	// surface a reader navigates by filename.
	for _, pkg := range []model.Package{
		{Package: "main", Path: "./cmd/zoo"},
		{Package: "zoo_test", Path: "./zoo", TestPackage: true},
	} {
		if issues := lint(t, document(pkg, misplaced("Elephant", "animals.go"), nil, nil)); len(issues) != 0 {
			t.Errorf("%s: reported %d issues", pkg.Package, len(issues))
		}
	}

	// A generator wrote the filename as much as the symbol, and neither is
	// anyone's choice.
	files := model.FileList{{Name: "animals.go", Generated: true}}
	if issues := lint(t, document(zoo, misplaced("Elephant", "animals.go"), nil, files)); len(issues) != 0 {
		t.Errorf("reported %d issues in a generated file", len(issues))
	}

	// A test file is not read by anyone looking for a symbol.
	if issues := lint(t, document(zoo, misplaced("Elephant", "animals_test.go"), nil, nil)); len(issues) != 0 {
		t.Errorf("reported %d issues in a test file", len(issues))
	}

	// An unexported type is not a surface either.
	if issues := lint(t, document(zoo, misplaced("elephant", "animals.go"), nil, nil)); len(issues) != 0 {
		t.Errorf("reported %d issues on an unexported type", len(issues))
	}

	// An interface and a named primitive declare a shape rather than a thing
	// with behaviour, and neither earns a file of its own.
	for _, kind := range []string{"interface", "string"} {
		decls := misplaced("Elephant", "animals.go")
		decls[0].Type = kind
		if issues := lint(t, document(zoo, decls, nil, nil)); len(issues) != 0 {
			t.Errorf("reported %d issues on a %s", len(issues), kind)
		}
	}
}

// TestLinterCountsWhatItRead covers the numbers, which are a split of the
// symbols read rather than a count of the findings.
func TestLinterCountsWhatItRead(t *testing.T) {
	types := model.DeclarationList{
		{Kind: model.TypeKind, Name: "Elephant", File: "elephant.go", Line: 5},
		{Kind: model.TypeKind, Name: "Giraffe", File: "animals.go", Line: 5},
		{Kind: model.TypeKind, Name: "Zebra", File: "animals.go", Line: 9},
	}

	report, err := grouping.New().Lint(context.Background(), document(zoo, types, nil, nil))
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	metric, known := report.Metrics().Packages["example.com/zoo"].(grouping.Metric)
	if !known {
		t.Fatalf("Metrics() holds no entry for example.com/zoo")
	}
	if metric.Symbols != 3 || metric.Passing != 1 || metric.Violations != 2 {
		t.Errorf("Metric = %+v, want 3 symbols, 1 passing, 2 violations", metric)
	}

	stats := report.Statistics()
	if len(stats) != 1 {
		t.Fatalf("Statistics() = %d tables, want 1", len(stats))
	}
	if len(stats[0].Labels) != 5 || len(stats[0].Rows) != 1 {
		t.Fatalf("Statistics() = %d labels and %d rows, want 5 and 1", len(stats[0].Labels), len(stats[0].Rows))
	}

	want := []string{"example.com/zoo", "3", "1", "2", "33.3%"}
	for i, cell := range want {
		if stats[0].Rows[0][i] != cell {
			t.Errorf("row[%d] = %q, want %q", i, stats[0].Rows[0][i], cell)
		}
	}
	if stats[0].Header == "" || stats[0].Footer == "" {
		t.Errorf("Statistics() = %q above and %q below, want both", stats[0].Header, stats[0].Footer)
	}
}

func TestLinterName(t *testing.T) {
	if got := grouping.New().Name(); got != grouping.Name {
		t.Errorf("Name() = %q, want %q", got, grouping.Name)
	}
}
