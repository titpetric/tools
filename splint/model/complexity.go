package model

type Complexity struct {
	Cognitive  int `json:"Cognitive" yaml:"Cognitive"`
	Cyclomatic int `json:"Cyclomatic" yaml:"Cyclomatic"`
	Lines      int `json:"Lines" yaml:"Lines"`

	// Coverage is the share of the statements that ran, from the Go coverage
	// profile --append-coverage names. It is zero for a parse that was given
	// none, and for a declaration holding no statements to measure.
	Coverage float64 `json:"Coverage,omitempty" yaml:"Coverage,omitempty"`
}
