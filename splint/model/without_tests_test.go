package model_test

import (
	"testing"

	"github.com/titpetric/tools/splint/model"
)

// withTests is a document of two packages: one holding a file and its test, and
// the test package the tests of it were compiled into.
func withTests() *model.DocumentRoot {
	imports := model.NewStringSet()
	imports.Add("thing.go", `"fmt"`)
	imports.Add("thing_test.go", `"testing"`)

	return &model.DocumentRoot{
		SchemaVersion: model.SchemaVersion,
		Packages: model.DefinitionList{
			{
				Package: model.Package{Package: "thing", ImportPath: "example.com/thing"},
				Files: model.FileList{
					{Name: "thing.go"},
					{Name: "thing_test.go", Test: true},
				},
				Imports: imports,
				Funcs: model.DeclarationList{
					{Kind: model.FuncKind, Name: "Do", File: "thing.go"},
					{Kind: model.FuncKind, Name: "TestDo", File: "thing_test.go"},
				},
			},
			{
				Package: model.Package{Package: "thing_test", ImportPath: "example.com/thing_test", TestPackage: true},
				Files:   model.FileList{{Name: "black_test.go", Test: true}},
				Funcs:   model.DeclarationList{{Kind: model.FuncKind, Name: "TestBlack", File: "black_test.go"}},
			},
		},
	}
}

// TestDocumentRoot_WithoutTests covers what a written document keeps: the
// packages somebody declared, and the files the toolchain compiles into them.
func TestDocumentRoot_WithoutTests(t *testing.T) {
	out := withTests().WithoutTests()

	if len(out.Packages) != 1 {
		t.Fatalf("WithoutTests() kept %d packages, want 1", len(out.Packages))
	}

	def := out.Packages[0]
	if len(def.Funcs) != 1 || def.Funcs[0].Name != "Do" {
		t.Errorf("WithoutTests() kept %v", def.Funcs)
	}
	if len(def.Files) != 1 || def.Files[0].Name != "thing.go" {
		t.Errorf("WithoutTests() kept the files %v", def.Files)
	}
	if _, ok := def.Imports["thing_test.go"]; ok {
		t.Error("WithoutTests() kept what a test file imports")
	}
	if _, ok := def.Imports["thing.go"]; !ok {
		t.Error("WithoutTests() dropped what the package imports")
	}
}

// TestDocumentRoot_WithoutTestsLeavesTheOriginal covers the copy: the linters
// read the document the parse produced, and they read it after it was written.
func TestDocumentRoot_WithoutTestsLeavesTheOriginal(t *testing.T) {
	in := withTests()
	in.WithoutTests()

	if len(in.Packages) != 2 {
		t.Errorf("WithoutTests() removed a package from the document it read: %d left", len(in.Packages))
	}
	if len(in.Packages[0].Funcs) != 2 {
		t.Errorf("WithoutTests() removed a declaration from the document it read: %d left", len(in.Packages[0].Funcs))
	}
	if len(in.Packages[0].Imports) != 2 {
		t.Errorf("WithoutTests() removed an import from the document it read: %d left", len(in.Packages[0].Imports))
	}
}

// TestDocumentRoot_WithoutTestsOfNothing covers the call a caller does not
// have to guard.
func TestDocumentRoot_WithoutTestsOfNothing(t *testing.T) {
	var doc *model.DocumentRoot
	if doc.WithoutTests() != nil {
		t.Error("WithoutTests() of nothing is something")
	}
}
