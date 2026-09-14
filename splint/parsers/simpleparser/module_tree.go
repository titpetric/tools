package simpleparser

import (
	"path/filepath"

	"github.com/titpetric/tools/splint/gomod"
	"github.com/titpetric/tools/splint/model"
)

// moduleTree answers which go.mod a directory builds under, which is the
// nearest one at or above it.
type moduleTree struct {
	root string

	// found is the module of each directory already asked about, so a tree of
	// fifty packages under one go.mod reads it once.
	found map[string]*model.Module

	// dirs is where the go.mod of each directory was found, which is what
	// tells a package of a nested module from one of the root module.
	dirs map[string]string
}

// newModuleTree returns a tree rooted at a directory.
func newModuleTree(root string) *moduleTree {
	return &moduleTree{root: root, found: map[string]*model.Module{}, dirs: map[string]string{}}
}

// of returns the module a directory builds under, and nil for a tree holding
// no go.mod.
func (t *moduleTree) of(dir string) *model.Module {
	if module, ok := t.found[dir]; ok {
		return module
	}

	module, moduleDir := t.read(dir)
	t.found[dir] = module
	t.dirs[dir] = moduleDir
	return module
}

// dirOf is the directory holding the go.mod a directory builds under, and is
// empty for a tree holding none.
func (t *moduleTree) dirOf(dir string) string {
	t.of(dir)
	return t.dirs[dir]
}

// read walks up from a directory to the first go.mod it finds. It stops at the
// filesystem root rather than at the parse root: a package parsed on its own
// still belongs to the module above it.
func (t *moduleTree) read(dir string) (*model.Module, string) {
	for {
		if module, err := gomod.Read(filepath.Join(dir, "go.mod")); err == nil {
			return module, dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, ""
		}
		dir = parent
	}
}
