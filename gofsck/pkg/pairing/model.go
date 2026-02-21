package pairing

// Report contains file-test pairing analysis results.
type Report struct {
	Files           int `json:"files"`
	Tests           int `json:"tests"`
	Paired          int `json:"paired"`
	StandaloneFiles int `json:"standalone_files"`
	StandaloneTests int `json:"standalone_tests"`
}
