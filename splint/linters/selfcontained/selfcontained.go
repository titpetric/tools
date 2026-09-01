// Package selfcontained measures what a file needs from the rest of its
// package.
//
// A declaration that reaches only its own imports and what is declared beside
// it in the file is extractable: everything it is built from is in one place,
// which is what `go build one_file.go` asks for. One that reaches a name
// declared in another file of the package is not, and the file it is in cannot
// move without that other file.
//
// The measure is a fuzzy one and says so. It reads the globals the parse
// recorded, which are the names a declaration reached that its own file does
// not declare, and counts the ones the package declares elsewhere. A name
// neither parser could resolve is a name the package does not declare, and it
// is not counted.
//
// A package of one file is left out. There is nothing for it to be coupled to,
// and every symbol in it would read as self contained for no reason worth
// reporting.
package selfcontained

import (
	"context"

	"github.com/titpetric/tools/splint/model"
)

// Name is how the linter is selected and how its measurements are labelled.
const Name = "selfcontained"

// Linter measures the file scope of every package.
type Linter struct{}

// New returns the linter.
func New() *Linter {
	return &Linter{}
}

// Name is the linter name.
func (l *Linter) Name() string {
	return Name
}

// Lint measures every package of two or more files and reports nothing.
//
// Coupling is a measurement rather than a finding: there is no share of it a
// package ought to have, and a package written as one unit in several files is
// as legitimate as one written as several.
func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	results := &Results{}

	for _, dir := range directories(root) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		measure(results, dir)
	}

	return *results, nil
}

// directory is every definition of one package directory, which is the package
// and the external test package beside it.
type directory struct {
	// path is the directory relative to the parse root, and importPath is what
	// the package it holds is imported as.
	path       string
	importPath string

	defs []*model.Definition
}

// directories groups the document by the directory each definition was read
// from, in the order the document lists them.
//
// A directory holding an external test package is two definitions of one
// package, and the files of both are files of that directory.
func directories(root *model.DocumentRoot) []*directory {
	var out []*directory
	at := map[string]*directory{}

	for _, def := range root.Packages {
		dir, known := at[def.Package.Path]
		if !known {
			dir = &directory{path: def.Package.Path, importPath: def.Package.ImportPath}
			at[def.Package.Path] = dir
			out = append(out, dir)
		}
		// The package the tests are for is what the directory is called, and
		// it is the shorter of the two import paths.
		if len(def.Package.ImportPath) < len(dir.importPath) {
			dir.importPath = def.Package.ImportPath
		}
		dir.defs = append(dir.defs, def)
	}

	return out
}

// measure counts one directory, and counts nothing where there is nothing to
// couple to.
func measure(results *Results, dir *directory) {
	files := readable(dir)
	if code(files) < 2 {
		return
	}

	where := index(dir.defs)
	pkg := results.pkg(dir.importPath)

	// Every file is recorded before anything is counted, so a file declaring
	// nothing is still a file of the package.
	for _, def := range dir.defs {
		for _, file := range def.Files {
			if _, known := files[file.Name]; known {
				results.file(pkg, at(def.Package, file.Name), file)
			}
		}
	}

	for _, def := range dir.defs {
		for _, decl := range def.DeclarationList() {
			file, known := files[decl.File]
			if !known {
				continue
			}

			metric := results.file(pkg, at(def.Package, decl.File), file)
			metric.Symbols++

			reaches := coupled(decl, where)
			if reaches {
				metric.Coupled++
			}

			switch decl.Kind {
			case model.TypeKind:
				metric.Types++
				if !reaches {
					metric.TypesSelf++
				}
			case model.FuncKind:
				metric.Funcs++
				if !reaches {
					metric.FuncsSelf++
				}
			}
		}
	}
}

// at is where a file of a package is, without a line: the measurement is of
// the file, and every declaration in it is counted against the same entry.
func at(pkg model.Package, name string) model.Position {
	position := (&model.Declaration{File: name}).Position(pkg)
	position.Line = 0
	return position
}

// readable are the files of a directory the measure reads, keyed by name.
//
// A generated file is left out. Its shape says something about the generator
// and nothing about how the package is written, which is the same reason
// filecheck leaves one out of its lengths.
func readable(dir *directory) map[string]model.File {
	out := map[string]model.File{}

	for _, def := range dir.defs {
		for _, file := range def.Files {
			if file.Generated {
				continue
			}
			out[file.Name] = file
		}
	}

	return out
}

// code is how many of the files are not tests, which is what decides whether a
// directory has anything to measure.
func code(files map[string]model.File) int {
	count := 0
	for _, file := range files {
		if !file.Test {
			count++
		}
	}
	return count
}
