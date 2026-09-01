package selfcontained_test

import (
	"testing"

	"github.com/titpetric/tools/splint/linters/selfcontained"
	"github.com/titpetric/tools/splint/model"
)

// document is a package of two files: store.go declares a type and a helper,
// and handler.go reaches both of them.
func document() *model.DocumentRoot {
	return &model.DocumentRoot{
		Packages: model.DefinitionList{{
			Package: model.Package{Package: "scope", ImportPath: "example.com/scope", Path: "./scope"},
			Files: model.FileList{
				{Name: "store.go", Lines: 20},
				{Name: "handler.go", Lines: 12},
			},
			Types: model.DeclarationList{
				{Kind: model.TypeKind, Name: "Store", File: "store.go"},
				{Kind: model.TypeKind, Name: "Handler", File: "handler.go", Globals: model.StringSet{"Store": nil}},
			},
			Funcs: model.DeclarationList{
				// A method of a type declared in the same file, reaching an
				// import and nothing else.
				{
					Kind: model.FuncKind, Name: "Get", Receiver: "*Store", File: "store.go",
					References: model.StringSet{"fmt": {"Sprintf"}},
				},
				{Kind: model.FuncKind, Name: "open", File: "store.go"},
				// A method of a type declared beside it, calling a helper from
				// the other file.
				{
					Kind: model.FuncKind, Name: "Serve", Receiver: "*Handler", File: "handler.go",
					Globals: model.StringSet{"open": nil},
				},
				// A method whose receiver is declared in the other file, which
				// is a reach whatever its body does.
				{Kind: model.FuncKind, Name: "Close", Receiver: "*Store", File: "handler.go"},
			},
		}},
	}
}

// TestCoupling covers what the measure counts: a global the package declares
// in another file, and a receiver declared in another file.
func TestCoupling(t *testing.T) {
	files := lint(t, document()).Metrics().Files

	store, ok := files["scope/store.go"].(selfcontained.Metric)
	if !ok {
		t.Fatalf("metrics hold %v", keys(files))
	}
	if store.Symbols != 3 || store.Coupled != 0 {
		t.Errorf("store.go = %d symbols, %d coupled, want 3 and 0", store.Symbols, store.Coupled)
	}
	if store.Types != 1 || store.TypesSelf != 1 || store.Funcs != 2 || store.FuncsSelf != 2 {
		t.Errorf("store.go = %#v", store)
	}

	handler := files["scope/handler.go"].(selfcontained.Metric)
	if handler.Symbols != 3 || handler.Coupled != 3 {
		t.Errorf("handler.go = %d symbols, %d coupled, want 3 and 3", handler.Symbols, handler.Coupled)
	}
	if handler.Coupling() != 100 {
		t.Errorf("handler.go reads %.1f%%, want 100", handler.Coupling())
	}
}

// TestCouplingIgnoresUnknownNames covers the fuzz the measure lives with: a
// global no file of the package declares is a local the parse did not see
// bound, and it couples nothing.
func TestCouplingIgnoresUnknownNames(t *testing.T) {
	root := document()
	root.Packages[0].Funcs[1].Globals = model.StringSet{"err": nil, "buf": nil}

	files := lint(t, root).Metrics().Files
	if store := files["scope/store.go"].(selfcontained.Metric); store.Coupled != 0 {
		t.Errorf("store.go = %d coupled, want 0: neither name is declared here", store.Coupled)
	}
}

// TestSingleFilePackage covers the package there is nothing to say about: one
// file is the whole scope, so everything in it is self contained by
// construction and none of it is measured.
func TestSingleFilePackage(t *testing.T) {
	root := document()
	root.Packages[0].Files = model.FileList{{Name: "store.go"}}

	report := lint(t, root)
	if !report.Metrics().Empty() {
		t.Errorf("a package of one file measured %v", keys(report.Metrics().Files))
	}
	if report.Statistics() != nil {
		t.Error("a package of one file drew a table")
	}
}

// TestGeneratedFilesAreNotMeasured covers the file nobody wrote: what it
// reaches says something about its generator.
func TestGeneratedFilesAreNotMeasured(t *testing.T) {
	root := document()
	root.Packages[0].Files[1].Generated = true
	// A third file, so the package still has two the measure reads.
	root.Packages[0].Files = append(root.Packages[0].Files, model.File{Name: "extra.go"})

	files := lint(t, root).Metrics().Files
	if _, measured := files["scope/handler.go"]; measured {
		t.Error("a generated file was measured")
	}
}

func keys(files map[string]any) []string {
	out := make([]string, 0, len(files))
	for key := range files {
		out = append(out, key)
	}
	return out
}
