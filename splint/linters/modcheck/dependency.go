package modcheck

// Dependency is one module the tree requires, and what it costs.
type Dependency struct {
	// Path and Version are the require line as go.mod writes it.
	Path    string `json:"Path" yaml:"Path"`
	Version string `json:"Version" yaml:"Version"`

	// Files is how many files import the module, Packages how many packages
	// they belong to, and Symbols how many distinct exported names the tree
	// reaches through it.
	//
	// The three answer different questions. Files is the blast radius of
	// removing it, Packages is how far it has spread, and Symbols is how much
	// of the module the tree actually uses: a dependency reached from forty files through
	// two symbols is a candidate for a wrapper, and one reached from two files
	// through forty is a dependency in earnest.
	//
	// The linter counts Files from the import list and Symbols from the names the
	// declarations of a package write, in a body and in a type position alike.
	// The two agree on what reaching means: a file naming nothing of a module
	// but one field's type imports it, so it counts, and the name it
	// writes there counts as a symbol.
	Files    int `json:"Files" yaml:"Files"`
	Packages int `json:"Packages" yaml:"Packages"`
	Symbols  int `json:"Symbols" yaml:"Symbols"`

	// Reach is how many further modules of the build this one requires, and
	// Weight is what it and every one of them weigh together.
	//
	// A size on its own understates what a dependency costs: a consumer takes on
	// the module and everything it drags in behind it. The linter counts the two
	// over the modules this build carries, so a dependency of a dependency
	// counts for each dependency that reaches it: the question each answers
	// is what this one costs, not how the total divides.
	Reach  int   `json:"Reach,omitempty" yaml:"Reach,omitempty"`
	Weight int64 `json:"Weight,omitempty" yaml:"Weight,omitempty"`

	// Indirect reports a dependency of a dependency, which nothing here
	// imports and nobody here chose.
	Indirect bool `json:"Indirect,omitempty" yaml:"Indirect,omitempty"`

	// Replaced is what a replace directive redirects the module to, and is
	// empty for a module resolved the way its version says.
	Replaced string `json:"Replaced,omitempty" yaml:"Replaced,omitempty"`

	// Size is the module zip in bytes and Latest the newest version the proxy
	// knows of. Both are blank when the proxy stays silent.
	Size   int64  `json:"Size,omitempty" yaml:"Size,omitempty"`
	Latest string `json:"Latest,omitempty" yaml:"Latest,omitempty"`

	// packages and symbols are the source of the counts, kept as sets
	// so a file importing a module twice counts once.
	packages map[string]bool
	symbols  map[string]bool

	// requires are the module paths this version's own go.mod requires, the
	// graph the reach walks over. The list is empty when the proxy stays
	// answer, and the reach then reads as unknown, never as zero.
	requires []string

	// testOnly reports a module no file outside the tests imports, which is a
	// dependency a consumer of this module never links.
	testOnly bool

	// blank reports a module some file imports for its side effect. A
	// registration is not a symbol and no caller can inline it, so a dependency
	// carrying one is not thin, whatever little of it the tree reaches by name.
	//
	// It answers whether a file writes an import for a side effect and not
	// whether that is worth reporting, so the exemptions the blank rule makes
	// are not made here: a file of a program may blank import a driver without
	// a reprimand for it, and the dependency is still one nothing can
	// inline.
	blank bool
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

// Shipped reports a dependency a consumer of this module links.
func (d Dependency) Shipped() bool {
	return !d.Indirect && !d.testOnly && d.Files > 0
}
