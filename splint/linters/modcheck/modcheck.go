// Package modcheck reports what a module depends on and what each dependency
// costs it.
//
// Size is the shallow half of that cost and the one every tool reports. The
// half that decides whether a consumer can remove a dependency is how far into the
// tree it reaches: a library pulled into one file through one symbol is cheap
// to drop at any weight, and one reached from forty files through twelve
// symbols holds the code up. Both halves are here, and the coupling half comes
// out of the document.
//
// This is the one linter that reaches outside the document. A size and a
// published date are properties of the artifact, and the source holds neither, so
// they come from the Go module proxy, and asking is the point of the check
// and is no option on it. A machine with no network reports the coupling
// and leaves the rest blank.
package modcheck

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/titpetric/tools/splint/gomod"
	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/modproxy"
)

// Name selects the linter and labels its issues.
const Name = "modcheck"

// The rules this linter reports under.
const (
	// RuleReplace is a replace directive, which is the one thing here that
	// fails a run.
	RuleReplace = "replace"

	// RuleUnused is a required module nothing imports.
	RuleUnused = "unused"

	// RuleMajors is two majors of one module required together.
	RuleMajors = "majors"

	// RuleThin is a dependency reached from one file through one symbol.
	RuleThin = "thin"

	// RuleBlank is a blank import outside the entry point of a program.
	RuleBlank = "blank"
)

// wiring reports a file that decides what a binary links, the place where a
// blank import belongs.
//
// A program is one: every file of package main is part of wiring it, and
// putting the driver in main.go or in serve.go is a preference and not a
// rule. So is a test file: a test binary is a program too, and a test that
// needs a driver registered registers it beside itself.
//
// What remains is a library, where an init runs because a package happened to
// sit in the link and every consumer of it pays for the decision.
func wiring(def *model.Definition, file string) bool {
	return def.Package.Package == "main" ||
		def.Package.TestPackage ||
		strings.HasSuffix(file, "_test.go")
}

// Linter reports the dependencies of a module and what they cost it.
type Linter struct {
	// Proxy answers what a dependency weighs and its age. A linter
	// built without one queries nobody, the setup a test uses.
	Proxy *modproxy.Client
}

// New returns the linter, querying the proxy the toolchain would.
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

	results := Results{deps: usage, modules: len(catalogue.Modules()), sums: catalogue.Sums()}
	results.ask(ctx, l.Proxy, usage)

	report(&results, catalogue, usage)
	blanks(&results, root, catalogue)

	// Every proxy answer is in by here, so the linter writes the sizes
	// once and not per module. A cache write that
	// fails is not a failure of the report.
	if l.Proxy != nil {
		_ = l.Proxy.Flush()
	}

	return results, nil
}

// blanks reports a package imported for its side effect from a file that wires
// no binary.
//
// A blank import runs an init and exports nothing else, so the set of
// packages the binary links decides what it does, and the code
// around it has no say. That is a decision for the program under build, and a
// library making it makes it for every consumer.
//
// The linter leaves a package of the tree under read alone. Where a project puts its own
// registrations is its own arrangement, and there is no convention to hold it
// to.
func blanks(results *Results, root *model.DocumentRoot, catalogue *gomod.Catalogue) {
	for _, def := range root.Packages {
		for _, file := range def.Imports.Keys() {
			if wiring(def, file) {
				continue
			}

			for _, literal := range def.Imports[file] {
				if !blankImport(literal) {
					continue
				}

				imported := importPath(literal)
				if imported == "embed" || catalogue.Owns(imported) {
					// A blank "embed" is what the compiler requires to embed
					// into a string, and is not a side effect at all.
					continue
				}

				results.add(Result{
					Rule:     RuleBlank,
					Severity: model.SeverityWarn,
					Symbol:   imported,
					Position: model.Position{Package: def.Package.Package, File: where(def, file)},
					Message: fmt.Sprintf("%s is imported for its side effect from %s, which wires no binary: what a program links is decided by the program",
						imported, file),
				})
			}
		}
	}
}

// where is the root relative path of a file of a package.
func where(def *model.Definition, file string) string {
	dir := strings.TrimPrefix(def.Package.Path, "./")
	if dir == "." {
		return file
	}
	return path.Join(dir, file)
}

// measure reads how far each dependency reaches into the tree.
//
// The linter counts a file once per dependency it imports, and a symbol once per
// dependency it comes from. The import set of a package maps the short name a
// reference sits under back to the path it names, which is the only way
// to attribute a reference: a reference says "chi.NewRouter", and which
// module chi is depends on what the file imported.
//
// A body or a declaration can reach a symbol, and the linter counts both. A body names it as a
// selector, which the document records as a reference; a declaration names it
// as a type, which the document records as the type string of a field, a
// parameter or a result and as no reference at all. Counting only the first
// reads a struct holding a *pdo.PDO and driving every query through it as a
// module reached once, the opposite of the truth.
//
// This pass also reads which file imports what, because that is the pass that sees
// every import literal. A side effect import is a property of the dependency
// and the linter records it there, so the rule that reads it does not depend on the rule
// that reports it having run first.
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

		// The short name a reference sits under, resolved to the path
		// the package imports it as.
		byName, _ := def.Imports.Map(def.Imports.All())

		for file, imports := range def.Imports {
			test := def.Package.TestPackage || strings.HasSuffix(file, "_test.go")

			// A file reaching two packages of one module is one file that
			// imports the module: "pdo" and "pdo/driver" side by side is one
			// place to change when the module goes. Counting the literals
			// instead reads a file importing a module and blank importing its
			// driver as two files, which is the one shape the side effect
			// exemption below exists for.
			counted := map[string]bool{}

			for _, literal := range imports {
				require, ok := catalogue.Owner(importPath(literal))
				if !ok {
					continue
				}
				dep := get(require)
				if !counted[require.Path] {
					counted[require.Path] = true
					dep.Files++
				}
				dep.packages[def.Package.ImportPath] = true
				if !test {
					dep.testOnly = false
				}
				if blankImport(literal) {
					dep.blank = true
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

		// Every declaration of the package, because each one can name a type:
		// them: a struct declares the fields, a func the parameters and the
		// results, and a var or a const the type it binds.
		for _, decl := range def.Order() {
			for _, typ := range typePositions(decl) {
				name, symbol, ok := qualified(typ)
				if !ok {
					continue
				}
				path, known := byName[name]
				if !known {
					continue
				}
				require, ok := catalogue.Owner(path)
				if !ok {
					continue
				}
				get(require).symbols[name+"."+symbol] = true
			}
		}
	}

	// A required module nothing imports still belongs in the report: a
	// require line that nothing reaches makes the finding.
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

// blankImport reports an import literal written for its side effect alone,
// which Definition.Imports records with the blank alias in front of the path.
func blankImport(literal string) bool {
	return strings.HasPrefix(literal, "_ ")
}

// typePositions are the type strings a declaration names, which is everything
// it reaches that is not a call.
//
// Type is the declared type for a var, a const or a named type, Fields are the
// fields of a struct and the methods of an interface, and Arguments and
// Returns are the parameter and result lists of a func. None of them is a
// reference: the document collects those from bodies.
func typePositions(decl *model.Declaration) []string {
	types := make([]string, 0, len(decl.Fields)+len(decl.Arguments)+len(decl.Returns)+1)

	if decl.Type != "" {
		types = append(types, decl.Type)
	}
	for _, field := range decl.Fields {
		if field != nil && field.Type != "" {
			types = append(types, field.Type)
		}
	}
	types = append(types, decl.Arguments...)
	types = append(types, decl.Returns...)

	return types
}

// qualified splits a type string into the name a file reaches a package by and
// the symbol of that package it names, and reports whether it names one.
//
// model.TypeRef is what reduces the type to the name: it takes off a pointer,
// a variadic, a slice or map layer and a generic instantiation, so "*pdo.PDO",
// "[]pdo.Row" and "map[string]*pdo.PDO" all come out as a qualified name. What
// it cannot reduce to one, a func type or a struct written out in place, names
// no single symbol and is not counted. model.IsPackageName is the identifier
// test both halves have to pass, and that test keeps a signature recorded as
// an interface method's type from reading as a qualified name.
func qualified(typ string) (name, symbol string, ok bool) {
	ref := model.TypeRef(typ)

	dot := strings.IndexByte(ref, '.')
	if dot < 0 {
		return "", "", false
	}

	name, symbol = ref[:dot], ref[dot+1:]
	if !model.IsPackageName(name) || !model.IsPackageName(symbol) {
		return "", "", false
	}
	return name, symbol, true
}

// report records what is worth saying out loud, beyond the counts.
func report(results *Results, catalogue *gomod.Catalogue, deps map[string]*Dependency) {
	for _, replace := range catalogue.Replaces() {
		// A replace is a build that does not reproduce anywhere else: what a
		// consumer resolves is not what the go.mod says. This is the one thing
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
			// An indirect require line is a dependency of a dependency, and
			// nothing here imports it. There is nothing to say about it.

		case dep.Files == 0:
			results.add(Result{
				Rule:     RuleUnused,
				Severity: model.SeverityWarn,
				Symbol:   dep.Path,
				Message: fmt.Sprintf("%s is required and nothing imports it, so either go mod tidy has not run or it is reached from a build this parse did not read",
					dep.Path),
			})

		case dep.blank:
			// A side effect import pulls a dependency in
			// for what it registers and not for what it exports, the way
			// a database driver works. No caller can inline
			// the registration, so how few of its names the tree
			// reaches says nothing.

		case dep.Files == 1 && dep.Symbols == 1:
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
