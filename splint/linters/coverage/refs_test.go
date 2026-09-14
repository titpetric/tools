package coverage_test

import (
	"testing"

	"github.com/titpetric/tools/splint/model"
)

// TestLinter_Lint_References covers the reference reading: a symbol a test
// reaches under another name is covered, however the test is named.
func TestLinter_Lint_References(t *testing.T) {
	client := &model.Declaration{Kind: model.TypeKind, Name: "Client", File: "x.go", Line: 3}
	closeMethod := &model.Declaration{Kind: model.FuncKind, Name: "Close", Receiver: "*Client", File: "x.go", Line: 7}
	open := &model.Declaration{Kind: model.FuncKind, Name: "Open", File: "x.go", Line: 11}

	def := definition(source, model.DeclarationList{client}, model.DeclarationList{closeMethod, open})

	// A blackbox test reaching Client.Close through a local: the reference
	// carries the method, dotted, under the import's short name.
	test := &model.Declaration{
		Kind:       model.FuncKind,
		Name:       "TestShutdown",
		File:       "x_test.go",
		Line:       1,
		References: model.StringSet{"x": []string{"Client.Close"}},
	}
	testDef := &model.Definition{
		Package: model.Package{
			Package:     "x_test",
			ImportPath:  "example.com/x_test",
			Path:        "./x",
			TestPackage: true,
		},
		Files:   model.FileList{{Name: "x_test.go", Test: true}},
		Imports: model.StringSet{"x_test.go": []string{`"example.com/x"`}},
		Funcs:   model.DeclarationList{test},
	}

	root := &model.DocumentRoot{Packages: model.DefinitionList{def, testDef}}

	issues := lint(t, root)
	if len(issues) != 1 {
		t.Fatalf("reported %d issues, want 1: %v", len(issues), issues)
	}
	if issues[0].Symbol != "Open" {
		t.Errorf("reported %q, want Open: the method and its type are referenced", issues[0].Symbol)
	}
	if want := "no test references this symbol, and none is named TestOpen"; issues[0].Message != want {
		t.Errorf("message = %q, want %q", issues[0].Message, want)
	}
}

// TestLinter_Lint_Globals covers the whitebox reading: a bare name a test
// reaches resolves against the package beside it.
func TestLinter_Lint_Globals(t *testing.T) {
	open := &model.Declaration{Kind: model.FuncKind, Name: "Open", File: "x.go", Line: 3}
	load := &model.Declaration{Kind: model.FuncKind, Name: "Load", File: "x.go", Line: 7}

	def := definition(source, nil, model.DeclarationList{open, load})

	test := &model.Declaration{
		Kind:    model.FuncKind,
		Name:    "TestEverything",
		File:    "x_test.go",
		Line:    1,
		Globals: model.StringSet{"Open": nil},
	}
	testDef := &model.Definition{
		Package: model.Package{
			Package:     "x_test",
			ImportPath:  "example.com/x_test",
			Path:        "./x",
			TestPackage: true,
		},
		Files: model.FileList{{Name: "x_test.go", Test: true}},
		Funcs: model.DeclarationList{test},
	}

	root := &model.DocumentRoot{Packages: model.DefinitionList{def, testDef}}

	issues := lint(t, root)
	if len(issues) != 1 {
		t.Fatalf("reported %d issues, want 1: %v", len(issues), issues)
	}
	if issues[0].Symbol != "Load" {
		t.Errorf("reported %q, want Load: Open is referenced bare", issues[0].Symbol)
	}
}
