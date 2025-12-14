package coverage

// Report contains symbol-test coverage analysis results.
type Report struct {
	Symbols         map[string][]string `json:"symbols"`
	Uncovered       []string            `json:"uncovered"`
	StandaloneTests []string            `json:"standalone_tests"`
	CoverageRatio   float64             `json:"coverage_ratio"`
}
