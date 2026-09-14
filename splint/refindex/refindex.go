// Package refindex answers which declarations reach a symbol, read off the
// references a document already carries.
//
// The index is built once from a document and asked many times: the coverage
// linter asks whether any test reaches a symbol, the pairing linter asks the
// same for every symbol of a file, and splint docs prints what the index
// holds for one symbol.
//
// The reading is the References and Globals of every function: a Reference
// resolves through the imports of the file that wrote it, and a Global
// resolves against the package beside it, which for a test definition is the
// product half of the same directory. A method recorded as Type.Method also
// marks its type, because a type whose method is reached is reached.
package refindex

import (
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// Key names one declared symbol: the import path of the package declaring it,
// and the symbol the way a reader refers to it, which is Func, Type or
// Type.Method.
type Key struct {
	ImportPath string
	Symbol     string
}

// Ref is one declaration that reaches a symbol.
type Ref struct {
	// Decl is the referencing function, and Pkg the package it sits in, which
	// is what says where it is.
	Decl *model.Declaration
	Pkg  model.Package

	// Test reports the reference coming from a test: a declaration in a
	// _test.go file, or one in a test package.
	Test bool
}

// Index is the reverse lookup: every symbol a document reaches, and the
// declarations that reach it.
type Index struct {
	refs map[Key][]Ref
	seen map[Key]map[*model.Declaration]bool
}

// Build reads every function of the document into the index.
func Build(root *model.DocumentRoot) *Index {
	ix := &Index{
		refs: map[Key][]Ref{},
		seen: map[Key]map[*model.Declaration]bool{},
	}

	// The product half of a directory is what a test's globals resolve
	// against: a test arrives as a definition of its own, and the bare names
	// it reaches are declared by the definition beside it.
	product := map[string]string{}
	for _, def := range root.Packages {
		if !def.Package.TestPackage {
			product[def.Package.Path] = def.Package.ImportPath
		}
	}

	for _, def := range root.Packages {
		owner, known := product[def.Package.Path]
		if !known {
			owner = strings.TrimSuffix(def.Package.ImportPath, "_test")
		}

		// The package wide map is the reading Clean splits References by, so
		// a key one file reaches through another file's import still
		// resolves. The file's own map is tried first: it is the scope the
		// reference was written in, and the one an alias collision cannot
		// cross.
		all, _ := def.Imports.Map(def.Imports.All())

		for _, decl := range def.Funcs {
			ref := Ref{
				Decl: decl,
				Pkg:  def.Package,
				Test: decl.IsTestScope() || def.Package.TestPackage,
			}

			var file map[string]string
			if imports := def.Imports.Get(decl.File); len(imports) > 0 {
				file, _ = def.Imports.Map(imports)
			}

			for short, symbols := range decl.References {
				long, ok := file[short]
				if !ok {
					long, ok = all[short]
				}
				if !ok {
					continue
				}
				for _, symbol := range symbols {
					ix.add(Key{ImportPath: long, Symbol: symbol}, ref)
					if typeName, _, isMethod := strings.Cut(symbol, "."); isMethod {
						ix.add(Key{ImportPath: long, Symbol: typeName}, ref)
					}
				}
			}

			for name, symbols := range decl.Globals {
				ix.add(Key{ImportPath: owner, Symbol: name}, ref)
				for _, symbol := range symbols {
					ix.add(Key{ImportPath: owner, Symbol: name + "." + symbol}, ref)
				}
			}
		}
	}

	return ix
}

// add records one declaration reaching one symbol, once: a declaration that
// reaches a symbol through two of its fields still reaches it once.
func (ix *Index) add(key Key, ref Ref) {
	if ix.seen[key] == nil {
		ix.seen[key] = map[*model.Declaration]bool{}
	}
	if ix.seen[key][ref.Decl] {
		return
	}
	ix.seen[key][ref.Decl] = true
	ix.refs[key] = append(ix.refs[key], ref)
}

// Refs is every declaration reaching the symbol, tests and product code both,
// in the order the document holds them.
func (ix *Index) Refs(key Key) []Ref {
	return ix.refs[key]
}

// TestRefs is every test declaration reaching the symbol.
func (ix *Index) TestRefs(key Key) []Ref {
	var found []Ref
	for _, ref := range ix.refs[key] {
		if ref.Test {
			found = append(found, ref)
		}
	}
	return found
}

// TestedDirect reports a symbol some test reaches by name: it appears in the
// references of a test function, however the test came to hold it.
func (ix *Index) TestedDirect(key Key) bool {
	for _, ref := range ix.refs[key] {
		if ref.Test {
			return true
		}
	}
	return false
}

// TestedTransitive reports a symbol some test reaches through the product
// code: directly, or through a chain of product declarations that reference
// it. It is the looser reading — a symbol reached by a tested caller is not
// necessarily executed by that caller's test — and no linter asks it by
// default.
func (ix *Index) TestedTransitive(key Key) bool {
	return ix.transitive(key, map[Key]bool{})
}

// transitive walks the callers of a symbol, each key at most once, so a cycle
// of product declarations terminates.
func (ix *Index) transitive(key Key, walking map[Key]bool) bool {
	if walking[key] {
		return false
	}
	walking[key] = true

	for _, ref := range ix.refs[key] {
		if ref.Test {
			return true
		}
		caller := Key{ImportPath: ref.Pkg.ImportPath, Symbol: ref.Decl.Symbol()}
		if ix.transitive(caller, walking) {
			return true
		}
	}
	return false
}
