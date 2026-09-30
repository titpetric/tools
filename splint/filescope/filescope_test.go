package filescope_test

import (
	"testing"

	"github.com/titpetric/tools/splint/filescope"
	"github.com/titpetric/tools/splint/model"
)

// defs is a package of two files: store.go declares a type and a helper, and
// handler.go reaches both of them.
func defs() []*model.Definition {
	return []*model.Definition{{
		Package: model.Package{Package: "scope", ImportPath: "example.com/scope", Path: "./scope"},
		Files: model.FileList{
			{Name: "store.go"},
			{Name: "handler.go"},
		},
		Types: model.DeclarationList{
			{Kind: model.TypeKind, Name: "Store", File: "store.go"},
			{Kind: model.TypeKind, Name: "Handler", File: "handler.go", Globals: model.StringSet{"Store": nil}},
		},
		Funcs: model.DeclarationList{
			{
				Kind: model.FuncKind, Name: "Get", Receiver: "*Store", File: "store.go",
				References: model.StringSet{"fmt": {"Sprintf"}},
			},
			{Kind: model.FuncKind, Name: "open", File: "store.go"},
			{
				Kind: model.FuncKind, Name: "Serve", Receiver: "*Handler", File: "handler.go",
				Globals: model.StringSet{"open": nil},
			},
			{Kind: model.FuncKind, Name: "Close", Receiver: "*Store", File: "handler.go"},
		},
	}}
}

// TestCoupled covers the two reaches: a global the package declares in another
// file, and a receiver declared in another file.
func TestCoupled(t *testing.T) {
	index := filescope.New(defs())

	for _, test := range []struct {
		name string
		decl *model.Declaration
		want bool
	}{
		{
			name: "a method of a type beside it",
			decl: &model.Declaration{Kind: model.FuncKind, Name: "Get", Receiver: "*Store", File: "store.go"},
			want: false,
		},
		{
			name: "a global from another file",
			decl: &model.Declaration{
				Kind: model.FuncKind, Name: "Serve", File: "handler.go",
				Globals: model.StringSet{"open": nil},
			},
			want: true,
		},
		{
			name: "a receiver from another file",
			decl: &model.Declaration{Kind: model.FuncKind, Name: "Close", Receiver: "*Store", File: "handler.go"},
			want: true,
		},
		{
			name: "a name the package does not declare",
			decl: &model.Declaration{
				Kind: model.FuncKind, Name: "Get", File: "store.go",
				Globals: model.StringSet{"err": nil, "buf": nil},
			},
			want: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := index.Coupled(test.decl); got != test.want {
				t.Errorf("Coupled() = %v, want %v", got, test.want)
			}
		})
	}
}

// TestSelfContained covers the file question, which is what the grouping linter
// asks: store.go reaches nothing outside itself, handler.go reaches both of
// store.go's names, and a file the package does not hold reaches nothing.
func TestSelfContained(t *testing.T) {
	index := filescope.New(defs())

	for file, want := range map[string]bool{
		"store.go":   true,
		"handler.go": false,
		"absent.go":  true,
	} {
		if got := index.SelfContained(file); got != want {
			t.Errorf("SelfContained(%q) = %v, want %v", file, got, want)
		}
		// Twice, because the answer is memoised.
		if got := index.SelfContained(file); got != want {
			t.Errorf("SelfContained(%q) = %v on the second ask", file, got)
		}
	}
}

// TestSelfContainedReadsEveryDefinition covers the external test package: it is
// a second definition of the same directory, and a test file reaching the
// package it tests does not make the file it tests coupled.
func TestSelfContainedReadsEveryDefinition(t *testing.T) {
	list := defs()
	list = append(list, &model.Definition{
		Package: model.Package{Package: "scope_test", ImportPath: "example.com/scope_test", Path: "./scope", TestPackage: true},
		Files:   model.FileList{{Name: "store_test.go", Test: true}},
		Funcs: model.DeclarationList{{
			Kind: model.FuncKind, Name: "TestStore", File: "store_test.go",
			Globals: model.StringSet{"Store": nil},
		}},
	})

	index := filescope.New(list)
	if !index.SelfContained("store.go") {
		t.Error("store.go reads as coupled once its test is in the document")
	}
	if index.SelfContained("store_test.go") {
		t.Error("the test file reaches Store and reads as self contained")
	}
}
