package visibility

// Report contains the exported and internal symbol counts of every package
// analyzed.
type Report struct {
	// Total is how many packages were measured, Passing how many crossed no
	// threshold, and Warnings the rest.
	Total    int `json:"total"`
	Passing  int `json:"passing"`
	Warnings int `json:"warnings"`

	Packages []PackageReport `json:"packages"`
}

// PackageReport counts what one package exports against what it keeps to
// itself, and how much of the package is the code behind the internal half.
type PackageReport struct {
	// Package is the package path relative to the module, with the module
	// root reported as "(root)".
	Package string `json:"package"`

	// ExportedTypes and InternalTypes count declared types by the case of
	// their name.
	ExportedTypes int `json:"exported_types"`
	InternalTypes int `json:"internal_types"`

	// ExportedFuncs and InternalFuncs count funcs and methods alike, by the
	// case of the name they are declared under.
	ExportedFuncs int `json:"exported_funcs"`
	InternalFuncs int `json:"internal_funcs"`

	// InternalLines is the code inside the bodies of internal funcs, and
	// Lines is the code of the package, both counted without blanks and
	// comments.
	InternalLines int `json:"internal_lines"`
	Lines         int `json:"lines"`

	// InternalRatio is InternalLines over Lines, as a percentage.
	InternalRatio float64 `json:"internal_ratio"`

	// Warnings are the thresholds this package crossed.
	Warnings []string `json:"warnings,omitempty"`
}

// Exported returns the exported symbols of the package, types and funcs
// together.
func (p PackageReport) Exported() int {
	return p.ExportedTypes + p.ExportedFuncs
}

// Internal returns the symbols the package keeps to itself.
func (p PackageReport) Internal() int {
	return p.InternalTypes + p.InternalFuncs
}

// OK reports whether the package crossed no threshold.
func (p PackageReport) OK() bool {
	return len(p.Warnings) == 0
}
