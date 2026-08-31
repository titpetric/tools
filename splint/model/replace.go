package model

// Replace is one replace directive of a go.mod.
//
// A replace changes what the build resolves a requirement to without changing
// what the requirement says, so it is recorded on its own: a release that
// starts or stops replacing a module changes what a consumer builds against
// while every version in the require block stays put.
type Replace struct {
	// Path is the module being replaced, and Version the single version the
	// directive applies to. An empty Version replaces every version of it.
	Path    string `json:"Path" yaml:"Path"`
	Version string `json:"Version,omitempty" yaml:"Version,omitempty"`

	// NewPath is what the module resolves to, which is either another module
	// path or a directory on disk. NewVersion is empty for a directory, which
	// carries no version.
	NewPath    string `json:"NewPath" yaml:"NewPath"`
	NewVersion string `json:"NewVersion,omitempty" yaml:"NewVersion,omitempty"`
}
