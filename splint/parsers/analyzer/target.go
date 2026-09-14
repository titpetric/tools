package analyzer

import (
	"golang.org/x/tools/go/packages"

	"github.com/titpetric/tools/splint/model"
)

// Target is one package the analyzer is about to read: the model entry it
// fills, and the loaded syntax it fills it from.
//
// The two are paired here rather than on model.Package, which is schema and
// reaches no parser: a document is a thing to encode, and a *packages.Package
// is not.
type Target struct {
	// Package is what the parse writes into.
	Package *model.Package

	// Syntax is the loaded package, and is what the collector walks.
	Syntax *packages.Package
}

// TargetList is the packages of one listing.
type TargetList []*Target

// Packages returns the model entries alone, for a caller that wants the list
// and not the syntax behind it.
func (t TargetList) Packages() []*model.Package {
	result := make([]*model.Package, 0, len(t))
	for _, target := range t {
		result = append(result, target.Package)
	}
	return result
}
