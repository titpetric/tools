package godoc

// Metric is what the linter counted in one package.
type Metric struct {
	// Exported is how many symbols were read, and Documented how many of them
	// carried a comment that says what it should.
	Exported   int `json:"Exported" yaml:"Exported"`
	Documented int `json:"Documented" yaml:"Documented"`

	// Missing, Format and Verbose are the findings, by rule.
	Missing int `json:"Missing" yaml:"Missing"`
	Format  int `json:"Format" yaml:"Format"`
	Verbose int `json:"Verbose" yaml:"Verbose"`
}
