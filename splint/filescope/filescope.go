// Package filescope answers what a file needs from the rest of its package.
//
// A declaration that reaches only its own imports and what the file declares
// beside it counts as extractable: everything it builds from sits in one
// place, and `go build one_file.go` needs exactly that. One that reaches a
// name declared in another file of the package is not, and the file holding
// it cannot move without that other file.
//
// The measure is a fuzzy one and says so. It reads the globals the parse
// recorded, which are the names a declaration reached that its own file does
// not declare, and counts the ones the package declares elsewhere. A name
// neither parser could resolve is a name the package does not declare, and it
// is not counted.
//
// Two linters ask. selfcontained counts the coupled declarations of every
// file, and grouping checks whether a whole file is free of them, because a
// file that builds alone moves as one unit and where a symbol sits inside it
// stays the file's own business.
package filescope

import (
	"github.com/titpetric/tools/splint/model"
)

// Index is every name a package declares, mapped to the file that declares it.
type Index struct {
	where map[string]string

	// contained memoises SelfContained: grouping asks once per symbol and a
	// file holding sixty of them would otherwise cost sixty walks.
	contained map[string]bool

	defs []*model.Definition
}

// New indexes the definitions of one package directory, which is the package
// and the external test package beside it.
//
// The name index leaves methods out. A caller reaches a method through its
// receiver, and the receiver is a type the index already holds.
func New(defs []*model.Definition) *Index {
	index := &Index{
		where:     map[string]string{},
		contained: map[string]bool{},
		defs:      defs,
	}

	for _, def := range defs {
		for _, decl := range def.DeclarationList() {
			if decl.Receiver != "" {
				continue
			}
			for _, name := range decl.GetNames() {
				if name == "" || name == "_" {
					continue
				}
				index.where[name] = decl.File
			}
		}
	}

	return index
}

// Coupled reports a declaration that reaches a name declared in another file
// of its own package.
//
// Two things reach one: a global the parse recorded, and the receiver of a
// method, since a method cannot move without the type it hangs off. A name
// the package does not declare is a local the parse did not see bound, or a
// builtin, and neither couples anything to anything.
func (i *Index) Coupled(decl *model.Declaration) bool {
	if receiver := model.TypeRef(decl.Receiver); receiver != "" {
		if file, known := i.where[receiver]; known && file != decl.File {
			return true
		}
	}

	for name := range decl.Globals {
		if file, known := i.where[name]; known && file != decl.File {
			return true
		}
	}

	return false
}

// SelfContained reports a file no declaration of which reaches another file of
// the package, which is a file `go build` compiles on its own.
func (i *Index) SelfContained(file string) bool {
	if answer, known := i.contained[file]; known {
		return answer
	}

	answer := true
	for _, def := range i.defs {
		for _, decl := range def.DeclarationList() {
			if decl.File != file {
				continue
			}
			if i.Coupled(decl) {
				answer = false
			}
		}
	}

	i.contained[file] = answer
	return answer
}
