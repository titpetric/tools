package model

// Sum is one module version go.sum records.
//
// go.sum is not the build list. It carries a hash for every version the module
// graph offered, whether or not the build selected it, and a version whose
// source was never needed is recorded by its go.mod alone. That is what makes
// it worth reading beside the go.mod: the go.mod says what a build resolves
// to, and go.sum says how much the resolution had to choose between.
type Sum struct {
	// Path is the module and Version the version of it recorded.
	Path    string `json:"Path" yaml:"Path"`
	Version string `json:"Version" yaml:"Version"`

	// Zip reports a version whose source is hashed as well as its go.mod,
	// which is a version the build downloads rather than one it read the
	// requirements of and passed over.
	Zip bool `json:"Zip,omitempty" yaml:"Zip,omitempty"`
}
