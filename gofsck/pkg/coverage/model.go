package coverage

// Report contains symbol-test coverage analysis results.
type Report struct {
	Symbols          map[string][]string `json:"symbols"`
	Covered          int                 `json:"covered"`
	Uncovered        []UncoveredSymbol   `json:"uncovered"`
	Constructors     int                 `json:"constructors"`
	StandaloneTests  []string            `json:"standalone_tests"`
	CoverageRatio    float64             `json:"coverage_ratio"`
	AdjustedCoverage float64             `json:"adjusted_coverage"`
	WantUnit         int                 `json:"want_unit_tests"`
	WantIntegration  int                 `json:"want_integration_tests"`
}

// UncoveredSymbol represents an exported symbol without test coverage.
type UncoveredSymbol struct {
	Symbol       string `json:"symbol"`
	ExpectedTest string `json:"expected_test"`
	TestKind     string `json:"test_kind"`
}
