package refindex_test

import (
	"testing"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/refindex"
)

// product is the package under test in every case below.
var product = model.Package{Package: "x", ImportPath: "example.com/x", Path: "./x"}

// definition wraps functions into the package a document holds them in.
func definition(pkg model.Package, funcs ...*model.Declaration) *model.Definition {
	return &model.Definition{Package: pkg, Funcs: model.DeclarationList(funcs)}
}

// blackbox is the test package beside the product, importing it by name.
func blackbox(funcs ...*model.Declaration) *model.Definition {
	def := definition(model.Package{
		Package:     "x_test",
		ImportPath:  "example.com/x_test",
		Path:        "./x",
		TestPackage: true,
	}, funcs...)
	def.Imports = model.StringSet{"x_test.go": []string{`"example.com/x"`}}
	return def
}

// whitebox is the test half of the product package: same directory, the
// parser's _test suffix on the names.
func whitebox(funcs ...*model.Declaration) *model.Definition {
	return definition(model.Package{
		Package:     "x_test",
		ImportPath:  "example.com/x_test",
		Path:        "./x",
		TestPackage: true,
	}, funcs...)
}

func testFunc(name string) *model.Declaration {
	return &model.Declaration{Kind: model.FuncKind, Name: name, File: "x_test.go", Line: 1}
}

func key(symbol string) refindex.Key {
	return refindex.Key{ImportPath: "example.com/x", Symbol: symbol}
}

// TestBuild_References covers the blackbox reach: a reference resolves
// through the imports of the file that wrote it, and a method marks its type
// as well as itself.
func TestBuild_References(t *testing.T) {
	test := testFunc("TestClose")
	test.References = model.StringSet{"x": []string{"Client.Close"}}

	index := refindex.Build(&model.DocumentRoot{Packages: model.DefinitionList{
		definition(product),
		blackbox(test),
	}})

	for _, symbol := range []string{"Client.Close", "Client"} {
		if !index.TestedDirect(key(symbol)) {
			t.Errorf("TestedDirect(%s) = false, want true", symbol)
		}
	}
	if index.TestedDirect(key("Open")) {
		t.Error("TestedDirect(Open) = true for a symbol nothing references")
	}
	if refs := index.TestRefs(key("Client.Close")); len(refs) != 1 || !refs[0].Test {
		t.Errorf("TestRefs(Client.Close) = %v, want one test reference", refs)
	}
}

// TestBuild_UnresolvableReference covers a reference under a name no import
// answers for: it resolves to nothing rather than to a wrong package.
func TestBuild_UnresolvableReference(t *testing.T) {
	test := testFunc("TestClose")
	test.References = model.StringSet{"zzz": []string{"Close"}}

	index := refindex.Build(&model.DocumentRoot{Packages: model.DefinitionList{
		definition(product),
		blackbox(test),
	}})

	if index.TestedDirect(key("Close")) {
		t.Error("TestedDirect(Close) = true from a reference no import resolves")
	}
}

// TestBuild_Globals covers the whitebox reach: a bare name and a Type.Method
// pair both resolve against the product half of the same directory.
func TestBuild_Globals(t *testing.T) {
	test := testFunc("TestOpen")
	test.Globals = model.StringSet{
		"Open":   nil,
		"Client": []string{"Close"},
	}

	index := refindex.Build(&model.DocumentRoot{Packages: model.DefinitionList{
		definition(product),
		whitebox(test),
	}})

	for _, symbol := range []string{"Open", "Client", "Client.Close"} {
		if !index.TestedDirect(key(symbol)) {
			t.Errorf("TestedDirect(%s) = false, want true", symbol)
		}
	}
}

// TestBuild_Transitive covers the looser reading: a symbol reached only by a
// tested caller is transitively tested and not directly, and a cycle of
// product declarations terminates.
func TestBuild_Transitive(t *testing.T) {
	a := &model.Declaration{Kind: model.FuncKind, Name: "A", File: "x.go", Globals: model.StringSet{"B": nil}}
	b := &model.Declaration{Kind: model.FuncKind, Name: "B", File: "x.go", Globals: model.StringSet{"A": nil}}

	test := testFunc("TestA")
	test.Globals = model.StringSet{"A": nil}

	index := refindex.Build(&model.DocumentRoot{Packages: model.DefinitionList{
		definition(product, a, b),
		whitebox(test),
	}})

	if index.TestedDirect(key("B")) {
		t.Error("TestedDirect(B) = true, want false: no test names B")
	}
	if !index.TestedTransitive(key("B")) {
		t.Error("TestedTransitive(B) = false, want true: A is tested and reaches B")
	}
	if index.TestedTransitive(key("C")) {
		t.Error("TestedTransitive(C) = true for a symbol nothing reaches")
	}
}

// TestBuild_ProductRefs covers the split: a product reference is a reference
// and not a test.
func TestBuild_ProductRefs(t *testing.T) {
	caller := &model.Declaration{Kind: model.FuncKind, Name: "Run", File: "x.go", Globals: model.StringSet{"Open": nil}}

	index := refindex.Build(&model.DocumentRoot{Packages: model.DefinitionList{
		definition(product, caller),
	}})

	if index.TestedDirect(key("Open")) {
		t.Error("TestedDirect(Open) = true from a product reference")
	}
	if refs := index.Refs(key("Open")); len(refs) != 1 || refs[0].Test {
		t.Errorf("Refs(Open) = %v, want one product reference", refs)
	}
	if refs := index.TestRefs(key("Open")); len(refs) != 0 {
		t.Errorf("TestRefs(Open) = %v, want none", refs)
	}
}
