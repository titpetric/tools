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
}
