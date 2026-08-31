package coverage

// Metric is what the linter counted in one package.
type Metric struct {
	// Exported is how many symbols a consumer can reach, and Covered how many
	// of them a test is named for.
	Exported int `json:"Exported" yaml:"Exported"`
	Covered  int `json:"Covered" yaml:"Covered"`

	// Constructors is how many of the exported symbols build a value. They are
	// counted apart because a test of a type covers the constructor that
	// returns it, so a package that is mostly constructors reads as better
	// covered than the tests it has would suggest.
	Constructors int `json:"Constructors" yaml:"Constructors"`

	// Uncovered is the findings, one per exported symbol no test is named for.
	Uncovered int `json:"Uncovered" yaml:"Uncovered"`
}
