package filecheck

// Metric is what the linter measured on one file.
//
// The metric is per file, not per package, because length is a property of a
// file: a package of ten short files and a package of one long one hold the
// same code, and only the second is worth reporting.
type Metric struct {
	// Lines is the code in the file, blanks and comments left out, and Size
	// is the bytes it takes on disk.
	Lines int `json:"Lines" yaml:"Lines"`
	Size  int `json:"Size" yaml:"Size"`

	// Test reports a file the toolchain compiles into the test binary. A long
	// test file reads differently from a long one beside it, so the linter counts
	// the two together and tells them apart.
	Test bool `json:"Test,omitempty" yaml:"Test,omitempty"`

	// Long reports the file the linter reported, and tells a
	// measurement the linter counted apart from one that is also a finding.
	Long bool `json:"Long,omitempty" yaml:"Long,omitempty"`
}
