package grouping

// Metric is what the linter counted in one package.
type Metric struct {
	// Symbols is how many exported symbols the rule read, which is what the
	// other two are a split of.
	Symbols int `json:"Symbols" yaml:"Symbols"`

	// Passing is how many of them sit in a file named for them, and Violations
	// how many do not.
	Passing    int `json:"Passing" yaml:"Passing"`
	Violations int `json:"Violations" yaml:"Violations"`

	// SelfContained is how many the rule left alone for sitting in a file that
	// compiles on its own. They are not read, so they are in none of the three
	// counts above.
	SelfContained int `json:"SelfContained,omitempty" yaml:"SelfContained,omitempty"`
}
