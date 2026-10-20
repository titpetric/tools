package imports

// Metric is what the linter counted in one package.
type Metric struct {
	// Files is how many files the package holds, and Names how many
	// short names their imports go by.
	Files int `json:"Files" yaml:"Files"`
	Names int `json:"Names" yaml:"Names"`

	// Collisions is how many of those names mean two things.
	Collisions int `json:"Collisions" yaml:"Collisions"`

	// Considered is how many files the linter read the import rules against, which
	// is every file of the package a generator did not write, and Formatted
	// how many of them hold the block they should.
	//
	// The two are zero for a parse that skipped the import
	// declarations: the linter read nothing, which says nothing about faults.
	Considered int `json:"Considered" yaml:"Considered"`
	Formatted  int `json:"Formatted" yaml:"Formatted"`

	// Unused is how many imports no name in their file reaches, and
	// Unresolved how many names reach a package nothing can place.
	Unused     int `json:"Unused" yaml:"Unused"`
	Unresolved int `json:"Unresolved" yaml:"Unresolved"`
}
