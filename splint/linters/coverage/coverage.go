// Package coverage reports an exported symbol with no test naming it.
//
// It is a port of the gofsck analyzer of the same name, reimplemented against
// the splint model: the check is the same idea and the reading is different,
// because a document is not a syntax tree.
package coverage

import (
	"context"
	"fmt"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// Name is how the linter is selected and how its issues are labelled.
const Name = "coverage"

// RuleUncovered is the one rule this linter reports under.
const RuleUncovered = "uncovered"

// Linter reports an exported symbol with no test naming it.
type Linter struct{}

// New returns the linter.
func New() *Linter {
	return &Linter{}
}

// Name is the linter name.
func (l *Linter) Name() string {
	return Name
}

// Lint reports the exported symbols of every package that no test is named
// for.
//
// A command exports nothing anyone reaches and a test package is not a surface
// anyone tests, so neither is read for symbols. Both are still read for tests,
// because a test of a package is usually declared in a package of its own.
func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	var results Results

	claimed := claims(root)

	for _, def := range root.Packages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if def.Package.Package == "main" || def.Package.TestPackage {
			continue
		}

		tested := claimed[def.Package.Path]

		var (
			found        []Result
			exported     int
			covered      int
			constructors int
		)
		for _, decls := range []model.DeclarationList{def.Types, def.Funcs} {
			for _, decl := range decls {
				if !reachable(def, decl) {
					continue
				}

				exported++
				if isConstructor(decl) {
					constructors++
				}
				if covers(tested, decl.Symbol()) {
					covered++
					continue
				}
				found = append(found, result(def.Package, decl))
			}
		}

		metric := results.count(def.Package, exported, covered, constructors)
		for _, result := range found {
			results.add(metric, result)
		}
	}

	return results, nil
}

// reachable reports a declaration somebody outside the package can name, which
// is what a test is expected for. A symbol declared in a test file is part of
// the test rather than of what it tests, and one in a generated file is
// nobody's to write a test for.
func reachable(def *model.Definition, decl *model.Declaration) bool {
	if !decl.IsExported() || decl.IsTestScope() {
		return false
	}
	if file, known := def.Files.Find(decl.File); known && file.Generated {
		return false
	}
	return true
}

// isConstructor reports a function that builds a value. It is counted apart
// because a constructor is reached through the test of the type it returns, so
// a package that is mostly constructors reads as better covered than it is.
func isConstructor(decl *model.Declaration) bool {
	return decl.Kind == model.FuncKind &&
		decl.Receiver == "" &&
		len(decl.Returns) > 0 &&
		strings.HasPrefix(decl.Name, "New")
}

// covers reports whether the tests of a package name the symbol.
//
// A method is looked for under both the names a test of it is written with:
// Client.Close is what TestClient_Close claims, and ClientClose is what
// TestClientClose claims, which is the same symbol with the dot taken out.
func covers(claimed map[string]bool, symbol string) bool {
	return claimed[symbol] || claimed[strings.ReplaceAll(symbol, ".", "")]
}

// claims indexes what the tests of a document say they test, keyed by the
// directory the package they are in sits in.
//
// The directory is the key rather than the import path because a test package
// arrives as a definition of its own: the tests of a package are not in the
// definition its symbols are in, the two carry different package names and
// different import paths, and where they sit on disk is what they share.
func claims(root *model.DocumentRoot) map[string]map[string]bool {
	claimed := map[string]map[string]bool{}

	for _, def := range root.Packages {
		for _, decl := range def.Funcs {
			if decl.Receiver != "" || !decl.IsTestScope() {
				continue
			}

			names := subjects(decl.Name)
			if len(names) == 0 {
				continue
			}

			path := def.Package.Path
			if claimed[path] == nil {
				claimed[path] = map[string]bool{}
			}
			for _, name := range names {
				claimed[path][name] = true
			}
		}
	}

	return claimed
}

// subjects are the symbols one test or benchmark function claims to cover.
//
// A test is named for what it tests and may say more after an underscore, so
// TestOpen and TestOpen_missingFile both test Open and TestClient_Close tests
// the method. TestClient claims NewClient as well, because a test of a type
// exercises the thing that builds it.
func subjects(name string) []string {
	base := strings.TrimPrefix(name, "Test")
	if base == name {
		base = strings.TrimPrefix(name, "Benchmark")
	}
	if base == name || base == "" {
		return nil
	}

	parts := strings.Split(base, "_")
	if parts[0] == "" {
		return nil
	}

	names := []string{parts[0], "New" + parts[0]}
	if len(parts) > 1 && parts[1] != "" {
		names = append(names, parts[0]+"."+parts[1])
	}
	return names
}

// result builds one finding, naming the test the symbol is missing.
func result(pkg model.Package, decl *model.Declaration) Result {
	symbol := decl.Symbol()
	return Result{
		Rule:     RuleUncovered,
		Symbol:   symbol,
		Position: decl.Position(pkg),
		Message:  fmt.Sprintf("exported symbol has no test named %s", wanted(symbol)),
	}
}

// wanted is the test a symbol is named for: TestOpen for a function,
// TestClient_Close for a method on Client.
func wanted(symbol string) string {
	return "Test" + strings.ReplaceAll(symbol, ".", "_")
}
