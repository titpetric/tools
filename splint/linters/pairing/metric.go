package pairing

// Metric is what the linter counted in one package.
type Metric struct {
	// Files is how many files the package holds that the toolchain leaves out
	// of a test binary, and Tests how many it compiles in. The linter counts the two
	// apart because a pair spans the halves and never sits within one.
	Files int `json:"Files" yaml:"Files"`
	Tests int `json:"Tests" yaml:"Tests"`

	// Paired is how many files have a test named after them.
	Paired int `json:"Paired" yaml:"Paired"`

	// StandaloneFiles is how many files nothing tests, each one a finding the
	// linter reports. StandaloneTests is how many tests name no file, which is
	// commonly a test outliving the file its author wrote it for.
	StandaloneFiles int `json:"StandaloneFiles" yaml:"StandaloneFiles"`
	StandaloneTests int `json:"StandaloneTests" yaml:"StandaloneTests"`

	// TestedElsewhere is how many files have no test beside them and every
	// exported symbol referenced by a test in another package, which excuses
	// the file and keeps it out of the report.
	TestedElsewhere int
}
