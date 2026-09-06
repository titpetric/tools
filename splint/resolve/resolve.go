// Package resolve answers what package a bare name refers to, from where it
// was written.
//
// A file reaching model.User with no import of model has to be told which
// model that is. goimports asks a global index and gets whichever one it
// indexed first, so a tree where /service1 and /service2 each hold a model
// package is a tree it formats wrong. The answer is source relative: from
// service2/storage, model is ../model and never service1/model.
//
// Nothing here reads a file. The document already carries every package of the
// tree with its directory and its import path, and the go.mod of every module
// it belongs to.
package resolve

import (
	"path"
	"sort"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// How a name resolved, which is what a message says when it has to explain
// itself and what a test asserts on.
const (
	// SourceAlias is a name the configuration named outright.
	SourceAlias = "alias"

	// SourceLocal is a package of the tree, found by walking up from the
	// directory the name was written in.
	SourceLocal = "local"

	// SourceTree is a path some other file of the tree already imports under
	// the name, and is what places a dependency without a module cache: one
	// file spelling out github.com/stretchr/testify/assert teaches every other
	// file what assert means.
	SourceTree = "tree"

	// SourceModule is a requirement of the go.mod whose last segment is the
	// name.
	SourceModule = "module"
)

// Result is what a name resolved to.
type Result struct {
	// Path is the import path, and is empty when nothing answered.
	Path string

	// Source is how it was found, one of the Source constants.
	Source string

	// Candidates are the paths that answered when more than one did, in which
	// case Path is empty: a formatter guessing between two would write the
	// wrong import half the time.
	Candidates []string
}

// Found reports a name that resolved to one path.
func (r Result) Found() bool {
	return r.Path != ""
}

// Ambiguous reports a name that more than one path answered to.
func (r Result) Ambiguous() bool {
	return r.Path == "" && len(r.Candidates) > 0
}

// Index is the document read as a resolver.
type Index struct {
	// aliases is what the configuration named, which wins over everything
	// else: it is the way to say what the tree could not.
	aliases map[string]string

	// byParent are the packages of the tree, keyed on the directory holding
	// them, so a walk up from a directory asks one map per level.
	byParent map[string][]*model.Definition

	// byName are the import paths the tree already reaches under each name.
	byName map[string][]string

	// requires are the module paths every go.mod of the tree requires, keyed
	// on the last segment of the path.
	requires map[string][]string

	// names are the package names of the tree's own packages, keyed on import
	// path. A path is reached by the name of the package at the end of it and
	// not by the last segment of the path, and the two differ often enough to
	// matter: github.com/goccy/go-yaml is reached as yaml.
	names map[string]string
}

// Name is the package name an import path is reached by, and whether it is
// known.
//
// It is known for a package of the tree, which the document names outright.
// For anything else the last segment of the path is a guess, and a guess that
// is not a Go identifier is wrong: no package is called go-yaml.
func (i *Index) Name(importPath string) (string, bool) {
	if name, ok := i.names[importPath]; ok {
		return name, true
	}

	base := model.BaseName(importPath)
	return base, model.IsPackageName(base)
}

// New builds the index over a document. The aliases are the configured hints,
// name to import path, and may be nil.
func New(root *model.DocumentRoot, aliases map[string]string) *Index {
	index := &Index{
		aliases:  aliases,
		byParent: map[string][]*model.Definition{},
		byName:   map[string][]string{},
		requires: map[string][]string{},
		names:    map[string]string{},
	}

	if root == nil {
		return index
	}

	for _, def := range root.Packages {
		if def.TestPackage {
			// A test package is compiled into a test binary and nothing
			// imports it, so it is not somewhere a name can resolve to.
			continue
		}
		if def.Package.ImportPath == "" {
			continue
		}
		parent := path.Dir(clean(def.Package.Path))
		index.byParent[parent] = append(index.byParent[parent], def)
		index.names[def.Package.ImportPath] = def.Package.Package

		for _, file := range def.Files {
			for _, spec := range file.ImportDecls.Specs() {
				if spec.Path == "C" {
					// "C" is cgo's pseudo package. Nothing resolves to it and
					// nothing may be written to import it.
					continue
				}
				index.addName(spec.Ref(), spec.Path)
			}
		}
	}

	for _, module := range root.Modules {
		for _, require := range module.Requires {
			index.addRequire(require.Path)
		}
	}

	return index
}

// addName records a path the tree reaches under a name.
func (i *Index) addName(name, importPath string) {
	if name == "" || importPath == "" {
		return
	}
	for _, known := range i.byName[name] {
		if known == importPath {
			return
		}
	}
	i.byName[name] = append(i.byName[name], importPath)
}

// addRequire records a module path under the name its last segment gives.
func (i *Index) addRequire(modulePath string) {
	name := model.BaseName(modulePath)
	if name == "" {
		return
	}
	for _, known := range i.requires[name] {
		if known == modulePath {
			return
		}
	}
	i.requires[name] = append(i.requires[name], modulePath)
}

// Lookup resolves a name as it was written in a package.
//
// The order is the order of confidence. A configured alias is a decision
// somebody made and wins outright. A package of the tree found by walking up
// from the directory is the next surest thing, and is what makes the same
// spelling mean two packages in two services. After that comes what the rest
// of the tree already writes, and last a requirement of the go.mod whose name
// matches.
func (i *Index) Lookup(def *model.Definition, name string) Result {
	if name == "" {
		return Result{}
	}

	if importPath, ok := i.aliases[name]; ok {
		return Result{Path: importPath, Source: SourceAlias}
	}

	if found := i.local(def, name); found != "" {
		return Result{Path: found, Source: SourceLocal}
	}

	if paths := i.byName[name]; len(paths) == 1 {
		return Result{Path: paths[0], Source: SourceTree}
	} else if len(paths) > 1 {
		return Result{Candidates: sorted(paths)}
	}

	if paths := i.requires[name]; len(paths) == 1 {
		return Result{Path: paths[0], Source: SourceModule}
	} else if len(paths) > 1 {
		return Result{Candidates: sorted(paths)}
	}

	return Result{}
}

// local is the package of the tree the name refers to, found by walking up
// from the directory the name was written in.
//
// The walk stops at the module the writing package belongs to. A package of
// another module is reached by importing that module, and a name that meant
// one of those would resolve differently depending on what else happened to be
// checked out beside it.
func (i *Index) local(def *model.Definition, name string) string {
	if def == nil {
		return ""
	}

	module := ""
	if def.Module != nil {
		module = def.Module.Path
	}

	for dir := clean(def.Package.Path); ; dir = path.Dir(dir) {
		for _, candidate := range i.byParent[dir] {
			if candidate.Package.Package != name {
				continue
			}
			if candidate == def {
				continue
			}
			if module != "" && (candidate.Module == nil || candidate.Module.Path != module) {
				continue
			}
			return candidate.Package.ImportPath
		}

		if dir == "." || dir == "/" || dir == "" {
			return ""
		}
	}
}

// clean is a package directory as the index keys on it: the "./x/y" the model
// records, without the leading dot, and "." for the root.
func clean(dir string) string {
	dir = strings.TrimPrefix(dir, "./")
	if dir == "" || dir == "." {
		return "."
	}
	return path.Clean(dir)
}

// sorted is a copy of the paths in order, so a message naming candidates names
// them the same way every run.
func sorted(paths []string) []string {
	out := make([]string, len(paths))
	copy(out, paths)
	sort.Strings(out)
	return out
}
