// Package visibility measures what a package exports against what it keeps.
//
// The package ports the gofsck analyzer of the same name onto
// the splint model: the check is the same idea and the reading is different,
// because a document is not a syntax tree.
//
// The linter reports nothing here as an issue. There is no
// share of internal code a package ought to carry: a parser keeps most of itself private
// and a data model exports most of itself, and both are fine. A threshold
// over this number would push every package to look like the same kind of
// package, so the linter reports the counts and leaves the reading to the
// people who own the package.
package visibility

import (
	"context"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// Name selects the linter and labels its issues.
const Name = "visibility"

// Linter measures what a package exports against what it keeps. A symbol's
// own name decides, so a method counts as a func, apart from its type.
type Linter struct{}

// New returns the linter.
func New() *Linter {
	return &Linter{}
}

// Name is the linter name.
func (l *Linter) Name() string {
	return Name
}

// Lint counts the exported and the internal half of every package it measures.
func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	var results Results

	for _, def := range root.Packages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if skip(def) {
			continue
		}
		count(results.count(def), def)
	}

	return results, nil
}

// skip reports a package the count would distort.
//
// A command exports nothing anyone imports, a test package is not a surface
// anyone reads, and an internal tree stays closed to importers outside the module
// whatever it exports, so Go has already answered the question this asks. A
// package left with no file anybody wrote has nothing to measure.
func skip(def *model.Definition) bool {
	if def.Package.Package == "main" || def.Package.TestPackage {
		return true
	}
	if internalPath(def.Package.ImportPath) || internalPath(def.Package.Path) {
		return true
	}
	return len(def.Files.Filter(measured)) == 0
}

// internalPath reports a path with an internal element in it.
func internalPath(path string) bool {
	for _, element := range strings.Split(path, "/") {
		if element == "internal" {
			return true
		}
	}
	return false
}

// count adds the declarations and the code of one package to its metric.
//
// The linter measures the package by what it ships, so the tests and the generated
// files stay out of both halves of the ratio: counting a generated file
// would say a package is private because a tool wrote a lot of it.
func count(metric *Metric, def *model.Definition) {
	for _, decl := range def.Types {
		if !shipped(def, decl) {
			continue
		}
		if decl.IsExported() {
			metric.ExportedTypes++
			continue
		}
		metric.InternalTypes++
	}

	for _, decl := range def.Funcs {
		if !shipped(def, decl) {
			continue
		}
		lines := bodyLines(decl)
		metric.Lines += lines

		if decl.IsExported() {
			metric.ExportedFuncs++
			continue
		}
		metric.InternalFuncs++
		metric.InternalLines += lines
	}
}

// bodyLines is the code one func occupies, which the model measures as the
// block from the func keyword to the brace that closes it.
func bodyLines(decl *model.Declaration) int {
	if decl.Complexity == nil {
		return 0
	}
	return decl.Complexity.Lines
}

// measured reports a file whose code counts toward the package.
func measured(file model.File) bool {
	return !file.Test && !file.Generated
}

// shipped reports a declaration the package ships. A declaration whose file
// the document does not record still counts, since the only thing that would
// rule it out is unknown.
func shipped(def *model.Definition, decl *model.Declaration) bool {
	if decl.IsTestScope() {
		return false
	}
	file, known := def.Files.Find(decl.File)
	return !known || !file.Generated
}
