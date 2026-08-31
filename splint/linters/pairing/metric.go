package pairing

// Metric is what the linter counted in one package.
type Metric struct {
	// Files is how many files the package holds that a test binary is not
	// built from, and Tests how many it holds that one is. The two are counted
	// apart because pairing is between the halves rather than within one.
	Files int `json:"Files" yaml:"Files"`
	Tests int `json:"Tests" yaml:"Tests"`

	// Paired is how many files have a test named after them.
	Paired int `json:"Paired" yaml:"Paired"`

	// StandaloneFiles is how many files nothing tests, which is what the
	// linter reports. StandaloneTests is how many tests name no file, which is
	// usually a test outliving the file it was written for.
	StandaloneFiles int `json:"StandaloneFiles" yaml:"StandaloneFiles"`
	StandaloneTests int `json:"StandaloneTests" yaml:"StandaloneTests"`
}
