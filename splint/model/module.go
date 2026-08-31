package model

// Module holds the go.mod of the module a package was extracted from.
//
// A package is only half of what a consumer takes on. The other half is the
// dependency set it drags in, and a release that moves a requirement to
// another major version, or replaces one with a fork, changes what it costs to
// depend on the package even though no symbol moved. Carrying the go.mod in
// the model is what lets two revisions be compared on that.
//
// The lists are sorted, so a go.mod that only had its blocks reordered
// produces the same model.
type Module struct {
	// Path is the module path, as the module directive declares it.
	Path string `json:"Path" yaml:"Path"`

	// GoVersion is the language version of the go directive. It bounds which
	// toolchains can build the module at all, so raising it drops support for
	// every consumer still on an older one.
	GoVersion string `json:"GoVersion,omitempty" yaml:"GoVersion,omitempty"`

	// Toolchain is the toolchain directive, and is empty for a module that
	// pins none.
	Toolchain string `json:"Toolchain,omitempty" yaml:"Toolchain,omitempty"`

	// Requires are the modules this one depends on.
	Requires []Require `json:"Requires,omitempty" yaml:"Requires,omitempty"`

	// Replaces are the replace directives.
	Replaces []Replace `json:"Replaces,omitempty" yaml:"Replaces,omitempty"`
}
