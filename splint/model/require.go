package model

// Require is one requirement of a go.mod.
type Require struct {
	// Path is the required module, and Version the version required of it.
	Path    string `json:"Path" yaml:"Path"`
	Version string `json:"Version" yaml:"Version"`

	// Indirect reports a requirement the module does not import itself, and
	// carries only to pin what a dependency of a dependency resolves to. It is
	// the "// indirect" comment go mod tidy writes. An indirect requirement is
	// bookkeeping, where a direct one is a decision.
	Indirect bool `json:"Indirect,omitempty" yaml:"Indirect,omitempty"`
}
