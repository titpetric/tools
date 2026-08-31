package modcheck

// Dependency is one module the tree requires, and what it costs.
type Dependency struct {
	// Path and Version are the requirement as go.mod writes it.
	Path    string `json:"Path" yaml:"Path"`
	Version string `json:"Version" yaml:"Version"`

	// Files is how many files import the module, Packages how many packages
	// they belong to, and Symbols how many distinct exported names are
	// reached through it.
	//
	// The three answer different questions. Files is the blast radius of
	// removing it, Packages is how far it has spread, and Symbols is how much
	// of it is actually used: a dependency reached from forty files through
	// two symbols is a candidate for a wrapper, and one reached from two files
	// through forty is a dependency in earnest.
	Files    int `json:"Files" yaml:"Files"`
	Packages int `json:"Packages" yaml:"Packages"`
	Symbols  int `json:"Symbols" yaml:"Symbols"`

	// Indirect reports a dependency of a dependency, which nothing here
	// imports and nobody here chose.
	Indirect bool `json:"Indirect,omitempty" yaml:"Indirect,omitempty"`

	// Replaced is what a replace directive redirects the module to, and is
	// empty for a module resolved the way its version says.
	Replaced string `json:"Replaced,omitempty" yaml:"Replaced,omitempty"`

	// Size is the module zip in bytes and Latest the newest version the proxy
	// knows of. Both are blank when the proxy was not reachable.
	Size   int64  `json:"Size,omitempty" yaml:"Size,omitempty"`
	Latest string `json:"Latest,omitempty" yaml:"Latest,omitempty"`

	// packages and symbols are what the counts are taken from, kept as sets
	// so a file importing a module twice counts once.
	packages map[string]bool
	symbols  map[string]bool

	// testOnly reports a module no file outside the tests imports, which is a
	// dependency a consumer of this module never links.
	testOnly bool
}

// Kind is how a reader classifies the dependency at a glance.
func (d Dependency) Kind() string {
	switch {
	case d.Replaced != "":
		return "replaced"
	case d.Indirect:
		return "indirect"
	case d.Files == 0:
		return "unused"
	case d.testOnly:
		return "test only"
	}
	return "direct"
}

// Shipped reports a dependency a consumer of this module links, which is every
// direct one that something outside the tests imports.
func (d Dependency) Shipped() bool {
	return !d.Indirect && !d.testOnly && d.Files > 0
}
