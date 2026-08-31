package wraphandler

// Metric is what the linter counted in one package.
type Metric struct {
	// Handlers is how many exported http.HandlerFunc signatures the package
	// offers, and Wrapped how many of them have an unexported function behind
	// them that a test can call.
	Handlers int `json:"Handlers" yaml:"Handlers"`
	Wrapped  int `json:"Wrapped" yaml:"Wrapped"`
}
