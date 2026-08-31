package model

import "sort"

// DefinitionList holds a list of Go packages.
type DefinitionList []*Definition

func (p DefinitionList) Walk(matchfn func(d *Definition)) {
	for _, decl := range p {
		matchfn(decl)
	}
}

func (p DefinitionList) Find(matchfn func(d *Definition) bool) *Definition {
	for _, decl := range p {
		if matchfn(decl) {
			return decl
		}
	}
	return nil
}

func (p DefinitionList) Filter(matchfn func(d *Definition) bool) (result []*Definition) {
	for _, decl := range p {
		if matchfn(decl) {
			result = append(result, decl)
		}
	}
	return
}

// Sort orders the packages by import path, then by whether they are the test
// half, so a package and its test package stay next to each other.
func (p DefinitionList) Sort() {
	sort.SliceStable(p, func(i, j int) bool {
		a, b := p[i], p[j]
		if a.ImportPath != b.ImportPath {
			return a.ImportPath < b.ImportPath
		}
		return !a.TestPackage && b.TestPackage
	})
}
