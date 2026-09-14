package collector

import (
	"go/ast"
	"go/types"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// majorVersion matches the major version a module path ends in, which is not
// part of the name the package is reached by: "example.com/thing/v2" is
// imported as "thing". It is the reading Imports.Map makes, repeated here so
// a name recorded on References is one the map resolves.
var majorVersion = regexp.MustCompile(`/v[0-9]+$`)

// resolveMethods records the methods a body reaches, which the reference scan
// cannot see: a selector on a local or on a field names no package, so a
// method reached through one is recorded nowhere without the type checker.
//
// A method on a type of the declaration's own package goes onto Globals in
// the shape Globals already documents: the package level name reached is the
// type, and the method is the name after the dot. A method on a type of an
// imported package goes onto References under the name the file imports the
// package by, as Type.Method. A file that does not import the declaring
// package records nothing, because References are what a declaration reaches
// through an import of its own file.
//
// A call through an interface records the interface's method, not any
// implementation's: which implementation runs is not the model's to say.
func (v *collector) resolveMethods(file *ast.File, filename string, fn *ast.FuncDecl, decl *Declaration) {
	if v.info == nil || v.pkg == nil || fn == nil || fn.Body == nil {
		return
	}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		obj, ok := v.info.Uses[sel.Sel].(*types.Func)
		if !ok {
			return true
		}

		// An instantiated generic reads as the declaration it was
		// instantiated from, which is the symbol the document holds.
		obj = obj.Origin()

		recv := obj.Signature().Recv()
		if recv == nil {
			// A package function: the name in front of the selector is the
			// package, which the reference scan already reads.
			return true
		}

		named := namedReceiver(recv.Type())
		if named == nil || obj.Pkg() == nil {
			return true
		}

		if obj.Pkg() == v.pkg {
			// Own package. A type the file declares itself is not recorded,
			// which is the same reading globals makes.
			if filepath.Base(v.fset.Position(named.Obj().Pos()).Filename) == filepath.Base(filename) {
				return true
			}
			decl.Globals.Add(named.Obj().Name(), obj.Name())
			return true
		}

		short := shortImportName(file, obj.Pkg().Path())
		if short == "" {
			return true
		}
		decl.References.Add(short, named.Obj().Name()+"."+obj.Name())
		return true
	})
}

// namedReceiver is the named type a method hangs off, with the pointer taken
// away. A receiver that is not a named type has no name a document carries,
// and reads as nil.
func namedReceiver(t types.Type) *types.Named {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	if named, ok := t.(*types.Named); ok {
		return named
	}
	return nil
}

// shortImportName is the name a file reaches an import by: the alias when it
// wrote one, and the base of the path with the major version taken off when
// it did not. A file that does not import the path, or imports it blank or
// dotted, reaches it by no name at all.
func shortImportName(file *ast.File, importPath string) string {
	for _, spec := range file.Imports {
		p := strings.Trim(spec.Path.Value, `"`)
		if p != importPath {
			continue
		}
		if spec.Name != nil {
			name := spec.Name.Name
			if name == "." || name == "_" {
				return ""
			}
			return name
		}
		if majorVersion.MatchString(p) {
			p = majorVersion.ReplaceAllString(p, "")
		}
		return path.Base(p)
	}
	return ""
}
