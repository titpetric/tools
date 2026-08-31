// Package linters is the registry: it imports every linter and hands back the
// list, so a program that wants all of them names this package alone.
package linters

import (
	"github.com/titpetric/tools/splint/linters/funcargs"
	"github.com/titpetric/tools/splint/linters/funcreturns"
	"github.com/titpetric/tools/splint/linters/godoc"
	"github.com/titpetric/tools/splint/linters/imports"
	"github.com/titpetric/tools/splint/model"
)

// All returns every linter, in the order a report reads them.
func All() []model.Linter {
	return []model.Linter{
		godoc.New(),
		imports.New(),
		funcargs.New(),
		funcreturns.New(),
	}
}

// Named returns the linters whose names are given, and the names that matched
// nothing. An empty selection is every linter.
func Named(names ...string) ([]model.Linter, []string) {
	if len(names) == 0 {
		return All(), nil
	}

	byName := make(map[string]model.Linter)
	for _, linter := range All() {
		byName[linter.Name()] = linter
	}

	var (
		selected []model.Linter
		unknown  []string
	)
	for _, name := range names {
		linter, ok := byName[name]
		if !ok {
			unknown = append(unknown, name)
			continue
		}
		selected = append(selected, linter)
	}
	return selected, unknown
}

// Names returns the name of every linter, in registry order.
func Names() []string {
	all := All()
	names := make([]string, 0, len(all))
	for _, linter := range all {
		names = append(names, linter.Name())
	}
	return names
}
