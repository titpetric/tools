package funcargs

// Metric is what the linter counted in one package.
type Metric struct {
	// Funcs is every func read, Considered the ones this check applies to,
	// and Passing the ones it had nothing to say about.
	Funcs      int `json:"Funcs" yaml:"Funcs"`
	Considered int `json:"Considered" yaml:"Considered"`
	Passing    int `json:"Passing" yaml:"Passing"`
}
