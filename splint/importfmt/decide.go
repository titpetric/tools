package importfmt

import (
	"fmt"
	"sort"
	"strings"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/resolve"
)

// Resolver answers what package a bare name refers to, and what name a package
// is reached by. resolve.Index is the one implementation; the interface is
// here so a test can answer for itself.
type Resolver interface {
	// Lookup resolves a name as written in a package.
	Lookup(def *model.Definition, name string) resolve.Result

	// Name is the package name an import path is reached by, and whether it
	// is known rather than guessed from the path.
	Name(importPath string) (string, bool)
}

// Unresolved is a name a file reaches that nothing could place.
type Unresolved struct {
	// Name is the name as the file writes it, before the dot.
	Name string

	// Symbols are what the file reached through it, which is what a message
	// quotes so a reader knows which line to look at.
	Symbols []string

	// Candidates are the paths that answered when more than one did, and are
	// empty when nothing answered at all.
	Candidates []string
}

// Message says what could not be placed and why, quoting a use so a reader
// knows which line to look at. It is what the linter reports and what the
// fixer says about a file it leaves alone.
func (u Unresolved) Message() string {
	reached := u.Name
	if len(u.Symbols) > 0 {
		reached += "." + u.Symbols[0]
	}

	if len(u.Candidates) > 0 {
		return fmt.Sprintf("%s reaches %s and %d packages answer to it: %s",
			reached, u.Name, len(u.Candidates), strings.Join(u.Candidates, ", "))
	}

	return fmt.Sprintf("%s reaches %s and no package of the tree, no file of it and no requirement is called that",
		reached, u.Name)
}

// Decision is what one file's import block should hold.
type Decision struct {
	// Specs are the imports the file should write, in the order it should
	// write them.
	Specs []model.ImportSpec

	// Added are the imports a name in the file needed and the file did not
	// have, and Removed the imports no name in the file reaches.
	Added   []model.ImportSpec
	Removed []model.ImportSpec

	// Unresolved are the names nothing could place. A file holding one of
	// these is not rewritten: the block would be written without an import the
	// file needs, and the result would not build.
	Unresolved []Unresolved

	// Tail is the comment lines the file's declarations hold that belong to no
	// import. They are written back at the end of the block.
	Tail string
}

// Text is the declaration the file should hold, and is empty for a file that
// should hold none.
func (d Decision) Text() string {
	return Render(d.Specs, d.Tail)
}

// Sound reports a decision that can be written. A file with a name nothing
// could place is left alone.
func (d Decision) Sound() bool {
	return len(d.Unresolved) == 0
}

// Changed reports the file's import block not being what it should be.
//
// The comparison is between two renderings of the same shape rather than
// between the file on disk and a string, so it answers the same whichever
// parser read the tree and whether or not the file was reached from disk at
// all. Whitespace inside a declaration is gofmt's business and does not show
// up here; the order of the imports, the groups, the aliases and the number of
// declarations do.
func (d Decision) Changed(all model.ImportDeclList) bool {
	// The import of "C" is pinned by the preamble above it. It is not part of
	// what a rewrite writes, so it is not part of what decides whether the
	// file needs one.
	current := all.Free()

	if len(current) > 1 && len(d.Specs) > 0 {
		// Two declarations in one file are one declaration written twice.
		return true
	}

	if len(current) == 1 {
		// The canonical block is parenthesised and holds one import per line.
		// import "fmt" and import ( "fmt" ) are the same imports written on
		// one line, and neither is the block.
		if len(d.Specs) > 0 && current[0].Line == current[0].EndLine {
			return true
		}
		// A declaration that holds nothing is a declaration to take out.
		if len(d.Specs) == 0 && len(current[0].Specs) == 0 {
			return true
		}
	}

	return Render(current.Specs(), current.Tail()) != d.Text()
}

// Decide returns what each file of a package should import, keyed by the base
// filename.
//
// A file with no import declarations and no name to place is not in the
// result, and neither is a generated one.
//
// The document is read for the test half of a package, which is a definition
// of its own holding only the test files. Without it a white box test reaching
// a name the package declares beside it reads as a missing import.
func Decide(root *model.DocumentRoot, def *model.Definition, idx Resolver, opts Options) map[string]Decision {
	if def == nil {
		return nil
	}

	declared := declaredNames(def)
	if def.TestPackage {
		for name := range declaredNames(sibling(root, def)) {
			declared[name] = true
		}
	}
	reached := reachedNames(def)

	out := map[string]Decision{}
	for _, file := range def.Files {
		if file.Generated {
			// A generated file is written by whatever generated it, which is
			// what goimports-reviser leaves alone too.
			continue
		}
		if file.Directives {
			// The file's physical line numbers are what its //line directives
			// rebase from. Moving a declaration moves them.
			continue
		}
		if len(file.ImportDecls) == 0 && len(reached[file.Name]) == 0 {
			continue
		}
		out[file.Name] = decideFile(def, file, declared, reached[file.Name], idx, opts, forbidden(def, file))
	}

	return out
}

// forbidden is the import path a file may not take on, which is the package
// the file says it is part of.
//
// A directory holds one package and up to two test scopes, and the model
// records both scopes under one definition. Only the file's own package clause
// says which it belongs to. A file writing "package whitebox" is compiled into
// whitebox and reaches Bar by name; adding an import of whitebox is an import
// cycle. A file writing "package whitebox_test" is a separate package and has
// to import it.
func forbidden(def *model.Definition, file model.File) string {
	self := strings.TrimSuffix(def.Package.ImportPath, "_test")
	if self == "" {
		return ""
	}

	// An empty clause is a parse that was not asked for it. Forbidding the
	// package's own path is then the reading that cannot produce a cycle.
	if file.Package == "" || file.Package == strings.TrimSuffix(def.Package.Package, "_test") {
		return self
	}

	return ""
}

// decideFile is one file's decision.
func decideFile(def *model.Definition, file model.File, declared map[string]bool, reached map[string][]string, idx Resolver, opts Options, forbid string) Decision {
	var decision Decision

	specs := file.ImportDecls.Free().Specs()
	kept := make([]model.ImportSpec, 0, len(specs))
	have := map[string]bool{}
	havePath := map[string]bool{}

	// A pinned declaration is not rewritten and its imports are still imports:
	// the file already reaches them, and nothing may be added to answer for a
	// name one of them already answers.
	for _, spec := range file.ImportDecls {
		if !spec.Cgo() {
			continue
		}
		for _, pinned := range spec.Specs {
			havePath[pinned.Path] = true
			if ref := pinned.Ref(); ref != "" {
				have[ref] = true
			}
		}
	}

	// guessing reports the file holding an import whose name the document does
	// not say. The name is then taken from the path, and the path does not
	// have to spell it: github.com/goccy/go-yaml is reached as yaml. A file
	// holding one of those cannot have its names accounted for, because any
	// name left over might be the one that import answers to.
	guessing := false

	for _, spec := range specs {
		havePath[spec.Path] = true

		if spec.Blank() || spec.Dot() {
			// A blank or a dot import is reached by no name, so no name can
			// stop reaching it. It stays until somebody deletes it by hand.
			kept = append(kept, spec)
			continue
		}

		ref, known := reachedBy(spec, idx)
		have[ref] = true
		guessing = guessing || !known

		// An import removed on a wrong guess is a file that no longer builds,
		// and one kept on a wrong guess is a line nobody notices.
		if !known || file.UsesName(ref) {
			kept = append(kept, spec)
			continue
		}

		decision.Removed = append(decision.Removed, spec)
	}

	for _, name := range sortedKeys(reached) {
		if have[name] || declared[name] {
			continue
		}

		found := idx.Lookup(def, name)

		// The name resolves to a path the file already imports, so the import
		// is there and is reached by a name the path does not spell. Nothing
		// is missing and nothing is added: a second import of one path under
		// two names compiles and says the file needs both.
		if found.Found() && havePath[found.Path] {
			have[name] = true
			continue
		}

		// The name resolves to the package this very file is part of, which
		// is a name the file already has and an import it cannot make.
		if found.Found() && found.Path == forbid {
			have[name] = true
			continue
		}

		if !found.Found() {
			if guessing {
				// The file holds an import whose name was guessed from its
				// path. This name is more likely to be that import than a
				// missing one, and reporting it would be an error against a
				// file that builds.
				continue
			}
			decision.Unresolved = append(decision.Unresolved, Unresolved{
				Name:       name,
				Symbols:    reached[name],
				Candidates: found.Candidates,
			})
			continue
		}

		spec := model.ImportSpec{Path: found.Path}
		if reached, _ := reachedBy(spec, idx); reached != name {
			// The package at the end of the path is not called what the file
			// writes, so the import has to say so.
			spec.Name = name
		}

		have[name] = true
		kept = append(kept, spec)
		decision.Added = append(decision.Added, spec)
	}

	decision.Specs = Canonical(kept, opts)
	decision.Tail = file.ImportDecls.Free().Tail()
	return decision
}

// reachedBy is the name a file reaches an import by, and whether that name is
// known rather than guessed.
//
// An alias is what the file writes, so it is known outright. Without one the
// name is the package's own, which the document says for a package of the tree
// and the resolver guesses from the path for anything else.
func reachedBy(spec model.ImportSpec, idx Resolver) (string, bool) {
	if spec.Name != "" {
		return spec.Name, true
	}
	if idx == nil {
		return model.BaseName(spec.Path), false
	}
	return idx.Name(spec.Path)
}

// Project is the module a document is of, which is the shortest module path it
// holds: a nested module sits under the one the tree is rooted at.
//
// It is what the project group defaults to for a tree whose configuration does
// not name one.
func Project(root *model.DocumentRoot) string {
	if root == nil || len(root.Modules) == 0 {
		return ""
	}

	shortest := root.Modules[0].Path
	for _, module := range root.Modules {
		if len(module.Path) < len(shortest) {
			shortest = module.Path
		}
	}

	return shortest
}

// sibling is the package a test package tests, which is the definition at the
// same directory that is not a test package.
func sibling(root *model.DocumentRoot, def *model.Definition) *model.Definition {
	if root == nil {
		return nil
	}
	for _, other := range root.Packages {
		if !other.TestPackage && other.Package.Path == def.Package.Path {
			return other
		}
	}
	return nil
}

// declaredNames are the package level names the package declares, across every
// file of it.
//
// A name a file reaches without importing it is either a package of the tree
// or something declared beside it, and this is what tells the two apart.
func declaredNames(def *model.Definition) map[string]bool {
	names := map[string]bool{}
	if def == nil {
		return names
	}
	for _, decl := range def.DeclarationList() {
		for _, name := range decl.GetNames() {
			if name != "" {
				names[name] = true
			}
		}
	}
	return names
}

// reachedNames are the names each file reaches through a selector, keyed by
// filename and then by name.
//
// Both sets a declaration carries are read. References are the names a parser
// matched to an import of the package, and Globals the ones it could not. The
// split is made against the package's imports rather than the file's, so a
// name one file imports and another reaches without importing is in
// References for both, and reading only Globals would miss the second file.
//
// Whether the name is really missing is settled by the caller, against the
// imports of the one file and the names the package declares.
//
// A key with no symbols is a bare identifier rather than a selector, and no
// import can be missing for one.
func reachedNames(def *model.Definition) map[string]map[string][]string {
	out := map[string]map[string][]string{}

	record := func(file, name string, symbols []string) {
		if name == "" || len(symbols) == 0 {
			return
		}
		if out[file] == nil {
			out[file] = map[string][]string{}
		}
		out[file][name] = append(out[file][name], symbols...)
	}

	for _, decl := range def.DeclarationList() {
		for name, symbols := range decl.References {
			record(decl.File, name, symbols)
		}
		for name, symbols := range decl.Globals {
			record(decl.File, name, symbols)
		}
	}

	for _, names := range out {
		for name, symbols := range names {
			sort.Strings(symbols)
			names[name] = unique(symbols)
		}
	}

	return out
}

// sortedKeys are the keys of a map in order, so a run reports the same names
// in the same order every time.
func sortedKeys(in map[string][]string) []string {
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// unique collapses the repeats out of a sorted list.
func unique(sorted []string) []string {
	out := sorted[:0]
	for i, value := range sorted {
		if i == 0 || value != sorted[i-1] {
			out = append(out, value)
		}
	}
	return out
}
