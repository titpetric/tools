package model

// Require is one require directive entry of a go.mod.
type Require struct {
	// Path is the required module, and Version the version required of it.
	Path    string `json:"Path" yaml:"Path"`
	Version string `json:"Version" yaml:"Version"`

	// Indirect reports a required module the module does not import itself,
	// and carries only to pin what a dependency of a dependency resolves to.
	// It reflects the "// indirect" comment go mod tidy writes. An indirect
	// entry is bookkeeping, where a direct one is a decision.
	Indirect bool `json:"Indirect,omitempty" yaml:"Indirect,omitempty"`
}
