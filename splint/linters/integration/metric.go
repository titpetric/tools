package integration

// Metric is what the linter counted in one package directory.
type Metric struct {
	// Tests is how many test functions a plain go test runs there, and
	// Crossing how many of them cross a process boundary.
	Tests    int `json:"Tests" yaml:"Tests"`
	Crossing int `json:"Crossing" yaml:"Crossing"`

	// Integration is how many test files sit behind the tag.
	Integration int `json:"Integration" yaml:"Integration"`
}
