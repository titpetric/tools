// Package modcheck reports what a module depends on and what each dependency
// costs it.
//
// Size is the shallow half of that cost and the one every tool reports. The
// half that decides whether a dependency can be removed is how far into the
// tree it reaches: a library pulled into one file through one symbol is cheap
// to drop whatever it weighs, and one reached from forty files through twelve
// symbols is load bearing. Both halves are here, and the coupling half comes
// out of the document.
//
// This is the one linter that reaches outside the document. A size and a
// published date are properties of the artifact rather than of the source, so
// they come from the Go module proxy, and asking is the point of the check
// rather than an option on it. A machine with no network reports the coupling
// and leaves the rest blank.
package modcheck

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/titpetric/tools/splint/gomod"
	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/modproxy"
)

// Name is how the linter is selected and how its issues are labelled.
const Name = "modcheck"

// The rules this linter reports under.
const (
	// RuleReplace is a replace directive, which is the one thing here that
	// fails a run.
	RuleReplace = "replace"

	// RuleUnused is a requirement nothing imports.
	RuleUnused = "unused"

	// RuleMajors is two majors of one module required together.
	RuleMajors = "majors"

	// RuleThin is a dependency reached from one file through one symbol.
	RuleThin = "thin"
)

// Linter reports the dependencies of a module and what they cost it.
type Linter struct {
	// Proxy answers what a dependency weighs and how old it is. A linter
	// built without one asks nobody, which is what a test does.
	Proxy *modproxy.Client
}

// New returns the linter, asking the proxy the toolchain would.
func New() *Linter {
	return &Linter{Proxy: modproxy.New()}
}

// Name is the linter name.
func (l *Linter) Name() string {
	return Name
}

// Lint reads the dependencies of the document and reports on them.
func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	catalogue := gomod.NewCatalogue(root)

	usage, err := measure(ctx, root, catalogue)
	if err != nil {
		return nil, err
	}

	results := Results{deps: usage, modules: len(catalogue.Modules())}
	results.ask(ctx, l.Proxy, usage)

	report(&results, catalogue, usage)
	return results, nil
}

// measure reads how far each dependency reaches into the tree.
//
// A file is counted once per dependency it imports, and a symbol once per
// dependency it comes from. The import set of a package maps the short name a
// reference is recorded under back to the path it names, which is the only way
// a reference can be attributed: a reference says "chi.NewRouter", and which
// module chi is depends on what the file imported.
func measure(ctx context.Context, root *model.DocumentRoot, catalogue *gomod.Catalogue) (map[string]*Dependency, error) {
	deps := map[string]*Dependency{}

	get := func(require model.Require) *Dependency {
		dep, known := deps[require.Path]
		if !known {
			dep = &Dependency{
				Path:     require.Path,
				Version:  require.Version,
				Indirect: require.Indirect,
				packages: map[string]bool{},
				symbols:  map[string]bool{},
				testOnly: true,
			}
			deps[require.Path] = dep
		}
		return dep
	}

	for _, def := range root.Packages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// The short name a reference is recorded under, resolved to the path
		// the package imports it as.
		byName, _ := def.Imports.Map(def.Imports.All())

		for file, imports := range def.Imports {
			test := def.Package.TestPackage || strings.HasSuffix(file, "_test.go")

			for _, literal := range imports {
				require, ok := catalogue.Owner(importPath(literal))
				if !ok {
					continue
				}
				dep := get(require)
				dep.Files++
				dep.packages[def.Package.ImportPath] = true
				if !test {
					dep.testOnly = false
				}
			}
		}

		for _, decl := range def.Funcs {
			for name, symbols := range decl.References {
				path, known := byName[name]
				if !known {
					continue
				}
				require, ok := catalogue.Owner(path)
				if !ok {
					continue
				}
				dep := get(require)
				for _, symbol := range symbols {
					dep.symbols[name+"."+symbol] = true
				}
			}
		}
	}

	// A requirement nothing imports still belongs in the report: that it is
	// required and unreached is the finding.
	for _, require := range catalogue.Requires() {
		get(require)
	}

	for _, dep := range deps {
		dep.Packages = len(dep.packages)
		dep.Symbols = len(dep.symbols)
		if dep.Files == 0 {
			dep.testOnly = false
		}
		if replace, ok := catalogue.Replaced(dep.Path); ok {
			dep.Replaced = replace.NewPath
		}
	}

	return deps, nil
}

// importPath is the path an import literal names, without the alias in front
// of it or the quotes around it.
func importPath(literal string) string {
	if at := strings.LastIndex(literal, " "); at >= 0 {
		literal = literal[at+1:]
	}
	return strings.Trim(literal, `"`)
}

// report records what is worth saying out loud rather than only counting.
func report(results *Results, catalogue *gomod.Catalogue, deps map[string]*Dependency) {
	for _, replace := range catalogue.Replaces() {
		// A replace is a build that does not reproduce anywhere else: what a
		// consumer resolves is not what the go.mod says. It is the one thing
		// here that fails a run.
		results.add(Result{
			Rule:     RuleReplace,
			Severity: model.SeverityError,
			Symbol:   replace.Path,
			Message: fmt.Sprintf("go.mod replaces %s with %s, so what this builds against is not what it requires",
				replace.Path, replace.NewPath),
		})
	}

	byBase := map[string][]string{}
	for _, dep := range sorted(deps) {
		if !dep.Indirect {
			byBase[gomod.Base(dep.Path)] = append(byBase[gomod.Base(dep.Path)], dep.Path)
		}

		switch {
		case dep.Indirect:
			// An indirect requirement is a dependency of a dependency, and
			// nothing here imports it. There is nothing to say about it.

		case dep.Files == 0:
			results.add(Result{
				Rule:     RuleUnused,
				Severity: model.SeverityWarn,
				Symbol:   dep.Path,
				Message: fmt.Sprintf("%s is required and nothing imports it, so either go mod tidy has not run or it is reached from a build this parse did not read",
					dep.Path),
			})

		case dep.Files == 1 && dep.Symbols == 1:
			// A dependency reaching no symbol at all is imported for what it
			// registers rather than for what it offers, which is how a
			// database driver is meant to be imported. There is nothing to
			// inline, so it is not reported.
			results.add(Result{
				Rule:     RuleThin,
				Severity: model.SeverityInfo,
				Symbol:   dep.Path,
				Message: fmt.Sprintf("%s is reached from one file through one symbol, which is usually cheaper to inline than to depend on",
					dep.Path),
			})
		}
	}

	for base, paths := range byBase {
		if len(paths) < 2 {
			continue
		}
		sort.Strings(paths)
		results.add(Result{
			Rule:     RuleMajors,
			Severity: model.SeverityWarn,
			Symbol:   base,
			Message: fmt.Sprintf("%s is required at two majors, %s, so both link in and the types of one do not satisfy the interfaces of the other",
				base, strings.Join(paths, " and ")),
		})
	}
}

// sorted returns the dependencies in path order.
func sorted(deps map[string]*Dependency) []*Dependency {
	out := make([]*Dependency, 0, len(deps))
	for _, dep := range deps {
		out = append(out, dep)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
