// Package selfcontained measures what a file needs from the rest of its
// package.
//
// A declaration that reaches only its own imports and what sits beside
// it in the file is extractable: everything it builds on is in one place,
// which is all `go build one_file.go` needs. One that reaches a name
// declared in another file of the package is not, and the file holding it cannot
// move without that other file. filescope answers that question,
// and the grouping linter asks it too.
//
// The linter leaves out a package of one file. There is nothing for it to couple to,
// and every symbol in it would read as self contained for no reason worth
// reporting.
package selfcontained

import (
	"context"

	"github.com/titpetric/tools/splint/filescope"
	"github.com/titpetric/tools/splint/model"
)

// Name selects the linter and labels its measurements.
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
// Coupling is a measurement and no finding: there is no share of it a
// package ought to have, and a package written as one unit in three files is
// as legitimate as one written as three.
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
	// the package it holds has as an import path.
	path       string
	importPath string

	defs []*model.Definition
}

// directories groups the document by the directory the linter read each definition
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
		// The directory takes its name from the package the tests are for, and
		// that package has the shorter of the two import paths.
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

	scope := filescope.New(dir.defs)
	pkg := results.pkg(dir.importPath)

	// The linter records every file before it counts anything, so a file declaring
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

			reaches := scope.Coupled(decl)
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

// at is where a file of a package sits, without a line: the measurement is of
// the file, and every declaration inside counts against the same entry.
func at(pkg model.Package, name string) model.Position {
	position := (&model.Declaration{File: name}).Position(pkg)
	position.Line = 0
	return position
}

// readable are the files of a directory the measure reads, keyed by name.
//
// The linter leaves out a generated file. Its shape says something about the generator
// and nothing about how the authors wrote the package, which is the same reason
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

// code is how many of the files are not tests; that count decides whether a
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
