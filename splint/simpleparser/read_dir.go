package simpleparser

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// readDir reads one directory into its definitions.
//
// A directory holds one package and, when it has tests, the test half of it.
// The two are separate definitions because they are separate scopes: what a
// test declares is not part of what the package ships.
func (p *Parser) readDir(root, moduleDir, modulePath, dir string) (model.DefinitionList, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") && !p.options.IncludeTests {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	// One definition per package scope: the package itself, and its tests.
	scopes := map[bool]*model.Definition{}

	// A package records one declaration per name, however many files declare
	// it. Two files each holding a blank "var _" declare the same name twice
	// and the package records it once, and the same goes for init.
	seen := map[bool]map[string]bool{false: {}, true: {}}

	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}

		src := newSource(name, data)
		parsed := scan(src)
		if parsed.Package == "" {
			continue
		}
		if !Included(name, data) {
			// A file behind a build constraint this build does not satisfy is
			// one the toolchain never compiles, so it is not part of the
			// package a reader would see.
			continue
		}

		isTest := strings.HasSuffix(name, "_test.go")
		def := scopes[isTest]
		if def == nil {
			importPath := importPath(modulePath, moduleDir, dir)
			def = &model.Definition{
				Package: model.Package{
					ID:          importPath,
					Package:     strings.TrimSuffix(parsed.Package, "_test"),
					ImportPath:  importPath,
					Path:        packagePath(root, moduleDir, dir),
					TestPackage: isTest,
				},
				Imports: model.NewStringSet(),
			}
			scopes[isTest] = def
		}

		def.Files = append(def.Files, model.File{
			Name:      name,
			Lines:     src.codeLines(),
			Size:      len(data),
			Generated: src.generated(),
			Test:      isTest,
		})

		merge(def, parsed, name, seen[isTest])
	}

	defs := make(model.DefinitionList, 0, len(scopes))
	for _, isTest := range []bool{false, true} {
		def, ok := scopes[isTest]
		if !ok {
			continue
		}
		splitGlobals(def)
		if isTest {
			// A white box test compiles into the package it tests, so the
			// listing names it the same. It is a different scope and is named
			// as one, which is what the ast parser records too.
			def.Package.Package += "_test"
			def.Package.ImportPath += "_test"
		}
		def.Sort()
		defs = append(defs, def)
	}

	return defs, nil
}

// merge folds one parsed file into the definition of its package, dropping
// what the package has already recorded under the same name.
func merge(def *model.Definition, parsed *file, name string, seen map[string]bool) {
	// A package comment on more than one file leaves the last of them
	// standing, which is what the collector records: it assigns the doc of
	// every file it walks, so the last one to have one wins.
	if parsed.Doc != "" {
		def.Doc = parsed.Doc
	}
	for _, literal := range parsed.Imports {
		def.Imports.Add(name, literal)
	}

	def.Types = append(def.Types, unseen(parsed.Types, seen, "decl")...)
	def.Consts = append(def.Consts, unseen(parsed.Consts, seen, "decl")...)
	def.Vars = append(def.Vars, unseen(parsed.Vars, seen, "decl")...)
	def.Funcs = append(def.Funcs, unseen(parsed.Funcs, seen, "func")...)
}

// unseen returns the declarations the package has not recorded yet, and marks
// the rest as recorded.
//
// A declaration is known by its name alone, so a blank "var _" is the same
// declaration in every file that writes one, and so is init. That is what the
// ast collector does with them, and a document that listed them per file would
// not compare against one that did not.
func unseen(decls model.DeclarationList, seen map[string]bool, scope string) model.DeclarationList {
	kept := make(model.DeclarationList, 0, len(decls))

	for _, decl := range decls {
		key := scope + ":" + decl.Symbol()
		if seen[key] {
			continue
		}
		seen[key] = true
		kept = append(kept, decl)
	}

	return kept
}

// importPath is the path a package is imported under, which is the module
// path plus the directory below the module root.
//
// A tree with no go.mod has no import path to give, since nothing can import
// what is not in a module.
func importPath(modulePath, moduleDir, dir string) string {
	if modulePath == "" {
		return ""
	}

	rel, err := filepath.Rel(moduleDir, dir)
	if err != nil || rel == "." {
		return modulePath
	}
	return modulePath + "/" + filepath.ToSlash(rel)
}

// packagePath names a directory the way the model names a package path.
//
// A package of the module the parse is rooted at is named from the root down,
// "." or "./frontend". A package of a module nested below it is named from the
// root as well, but with no separator after the dot: "generic" is a module of
// its own under this root and its packages read ".generic" and ".generic/sub".
//
// The two shapes are not a design, they are what "go-fsck extract" writes, and
// a document that spelled them differently would not compare against one.
func packagePath(root, moduleDir, dir string) string {
	if moduleDir == "" || moduleDir == root {
		rel, err := filepath.Rel(root, dir)
		if err != nil || rel == "." {
			return "."
		}
		return "./" + filepath.ToSlash(rel)
	}

	moduleRel, err := filepath.Rel(root, moduleDir)
	if err != nil {
		return "."
	}
	packageRel, err := filepath.Rel(moduleDir, dir)
	if err != nil || packageRel == "." {
		packageRel = ""
	}

	return "." + filepath.ToSlash(filepath.Join(moduleRel, packageRel))
}

// splitGlobals moves every reference the package does not import into Globals.
//
// The collector asks the same question the same way: a name that the package
// import table does not answer for is not a package, whatever it looked like
// in the body. The table is package wide, so the split cannot be made until
// every file of the package has been read.
func splitGlobals(def *model.Definition) {
	imports, _ := def.Imports.Map(def.Imports.All())

	for _, decl := range def.DeclarationList() {
		for name, symbols := range decl.References {
			if _, ok := imports[name]; ok {
				continue
			}
			if decl.Globals == nil {
				decl.Globals = model.NewStringSet()
			}
			decl.Globals.Add(name, symbols...)
			delete(decl.References, name)
		}
		if len(decl.References) == 0 {
			decl.References = nil
		}
	}
}
