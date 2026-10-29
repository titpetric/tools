package gomod

import (
	"sort"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// Catalogue is what a module depends on, and the packages each dependency
// holds.
//
// A document records the go.mod of every module it read and the import paths
// every file reaches, and the two are not the same namespace: a require names
// a module and an import names a package inside one. This is the map between
// them, and it lets a reader see what a dependency actually costs on top of
// the bare fact that a go.mod requires it.
type Catalogue struct {
	// modules are the modules the document came from, keyed by path.
	modules map[string]*model.Module

	// requires are every require line of those modules, keyed by module path.
	requires map[string]model.Require

	// replaces are every replace directive, keyed by the path it replaces.
	replaces map[string]model.Replace

	// paths are the required module paths, longest first, so a lookup of an
	// import path finds the most specific module that could hold it.
	paths []string
}

// NewCatalogue reads the modules of a document.
func NewCatalogue(doc *model.DocumentRoot) *Catalogue {
	c := &Catalogue{
		modules:  map[string]*model.Module{},
		requires: map[string]model.Require{},
		replaces: map[string]model.Replace{},
	}
	if doc == nil {
		return c
	}

	for _, module := range doc.Modules {
		if module == nil {
			continue
		}
		c.modules[module.Path] = module

		for _, require := range module.Requires {
			// A module required by two of the modules read is one dependency.
			// The catalogue keeps the direct require: something imports it
			// directly, and that is what a reader is deciding about.
			if known, seen := c.requires[require.Path]; !seen || (known.Indirect && !require.Indirect) {
				c.requires[require.Path] = require
			}
		}
		for _, replace := range module.Replaces {
			c.replaces[replace.Path] = replace
		}
	}

	for path := range c.requires {
		c.paths = append(c.paths, path)
	}
	sort.Slice(c.paths, func(i, j int) bool {
		if len(c.paths[i]) != len(c.paths[j]) {
			return len(c.paths[i]) > len(c.paths[j])
		}
		return c.paths[i] < c.paths[j]
	})

	return c
}

// Owner returns the required module that serves an import path, matched on a
// path boundary with the longest module path first.
func (c *Catalogue) Owner(importPath string) (model.Require, bool) {
	for _, path := range c.paths {
		if importPath == path || strings.HasPrefix(importPath, path+"/") {
			return c.requires[path], true
		}
	}
	return model.Require{}, false
}

// Requires returns every dependency, in path order.
func (c *Catalogue) Requires() []model.Require {
	out := make([]model.Require, 0, len(c.requires))
	for _, path := range c.paths {
		out = append(out, c.requires[path])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Sums returns every version go.sum records once, in path and version order.
func (c *Catalogue) Sums() []model.Sum {
	var out []model.Sum
	at := map[string]int{}

	for _, module := range c.Modules() {
		for _, sum := range module.Sums {
			key := sum.Path + "@" + sum.Version
			if index, seen := at[key]; seen {
				out[index].Zip = out[index].Zip || sum.Zip
				continue
			}
			at[key] = len(out)
			out = append(out, sum)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Version < out[j].Version
	})

	return out
}

// Replaces returns every replace directive, in path order.
func (c *Catalogue) Replaces() []model.Replace {
	out := make([]model.Replace, 0, len(c.replaces))
	for _, replace := range c.replaces {
		out = append(out, replace)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Replaced returns the directive redirecting a module, and whether one does.
func (c *Catalogue) Replaced(path string) (model.Replace, bool) {
	replace, ok := c.replaces[path]
	return replace, ok
}

// Modules returns the modules the document came from, in path order.
func (c *Catalogue) Modules() []*model.Module {
	out := make([]*model.Module, 0, len(c.modules))
	for _, module := range c.modules {
		out = append(out, module)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Owns reports whether a path belongs to one of the modules the document came
// from, and so tells code of the tree apart from a dependency.
func (c *Catalogue) Owns(importPath string) bool {
	for path := range c.modules {
		if importPath == path || strings.HasPrefix(importPath, path+"/") {
			return true
		}
	}
	return false
}

// Base is a module path without its major version suffix: "example.com/x/v2"
// is "example.com/x", so the majors of one module answer to one name.
func Base(path string) string {
	at := strings.LastIndex(path, "/")
	if at < 0 {
		return path
	}

	suffix := path[at+1:]
	if len(suffix) < 2 || suffix[0] != 'v' {
		return path
	}
	for _, r := range suffix[1:] {
		if r < '0' || r > '9' {
			return path
		}
	}

	return path[:at]
}
