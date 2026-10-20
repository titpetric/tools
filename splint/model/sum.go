package model

// Sum is one module version go.sum records.
//
// go.sum is not the build list. It carries a hash for every version the module
// graph offered, whether the build selected it or not, and it records a
// version whose source the build never needs by its go.mod alone. That makes
// it worth reading beside the go.mod: the go.mod says what a build resolves
// to, and go.sum says how much the resolution had to choose between.
type Sum struct {
	// Path is the module and Version the version of it recorded.
	Path    string `json:"Path" yaml:"Path"`
	Version string `json:"Version" yaml:"Version"`

	// Zip reports a version with a hash of its source as well as its go.mod:
	// a version the build downloads, as distinct from one it read the
	// requirements of and passed over.
	Zip bool `json:"Zip,omitempty" yaml:"Zip,omitempty"`
}
