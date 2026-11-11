package godoc

// Metric is what the linter counted in one package.
type Metric struct {
	// Exported is how many symbols the linter read. Documented is how many of
	// them held a comment that says what it should.
	Exported   int `json:"Exported" yaml:"Exported"`
	Documented int `json:"Documented" yaml:"Documented"`

	// The findings, by rule.
	Missing   int `json:"Missing" yaml:"Missing"`
	Format    int `json:"Format" yaml:"Format"`
	Verbose   int `json:"Verbose" yaml:"Verbose"`
	Outweighs int `json:"Outweighs" yaml:"Outweighs"`
	Restates  int `json:"Restates" yaml:"Restates"`
	Internals int `json:"Internals" yaml:"Internals"`
}
