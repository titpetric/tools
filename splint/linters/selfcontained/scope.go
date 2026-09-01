package selfcontained

import (
	"github.com/titpetric/tools/splint/model"
)

// index maps every name a package declares to the file that declares it.
//
// A method is left out. It is reached through its receiver rather than by
// name, and the receiver is a type the index already holds.
func index(defs []*model.Definition) map[string]string {
	where := map[string]string{}

	for _, def := range defs {
		for _, decl := range def.DeclarationList() {
			if decl.Receiver != "" {
				continue
			}
			for _, name := range decl.GetNames() {
				if name == "" || name == "_" {
					continue
				}
				where[name] = decl.File
			}
		}
	}

	return where
}

// coupled reports a declaration that reaches a name declared in another file
// of its own package, and is what the measure counts.
//
// Two things reach one: a global the parse recorded, and the receiver of a
// method, since a method cannot be moved without the type it hangs off. A name
// the package does not declare is a local the parse did not see bound, or a
// builtin, and neither couples anything to anything.
func coupled(decl *model.Declaration, where map[string]string) bool {
	if receiver := model.TypeRef(decl.Receiver); receiver != "" {
		if file, known := where[receiver]; known && file != decl.File {
			return true
		}
	}

	for name := range decl.Globals {
		if file, known := where[name]; known && file != decl.File {
			return true
		}
	}

	return false
}
