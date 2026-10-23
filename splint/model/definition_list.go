package model

import (
	"sort"
)

// DefinitionList holds a list of Go packages, a package and its test package
// next to each other once sorted.
type DefinitionList []*Definition

// Walk calls matchfn for each package.
func (p DefinitionList) Walk(matchfn func(d *Definition)) {
	for _, decl := range p {
		matchfn(decl)
	}
}

// Find returns the first package matchfn accepts,
// or nil.
func (p DefinitionList) Find(matchfn func(d *Definition) bool) *Definition {
	for _, decl := range p {
		if matchfn(decl) {
			return decl
		}
	}
	return nil
}

// Filter is the packages matchfn accepts.
func (p DefinitionList) Filter(matchfn func(d *Definition) bool) (result []*Definition) {
	for _, decl := range p {
		if matchfn(decl) {
			result = append(result, decl)
		}
	}
	return
}

// Sort orders the packages by import path, the test package second.
func (p DefinitionList) Sort() {
	sort.SliceStable(p, func(i, j int) bool {
		a, b := p[i], p[j]
		if a.ImportPath != b.ImportPath {
			return a.ImportPath < b.ImportPath
		}
		return !a.TestPackage && b.TestPackage
	})
}
