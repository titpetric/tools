package model

type Complexity struct {
	Cognitive  int `json:"Cognitive" yaml:"Cognitive"`
	Cyclomatic int `json:"Cyclomatic" yaml:"Cyclomatic"`
	Lines      int `json:"Lines" yaml:"Lines"`

	// Exits is how many ways a function leaves: each return, each panic,
	// os.Exit and log.Fatal call, and the end of a body that runs off its
	// closing brace. It holds zero for anything but a function.
	Exits int `json:"Exits,omitempty" yaml:"Exits,omitempty"`

	// Coverage is the share of the statements that ran, from the Go coverage
	// profile --append-coverage names. It holds zero for a parse with no
	// profile, and for a declaration holding no statements to measure.
	Coverage float64 `json:"Coverage,omitempty" yaml:"Coverage,omitempty"`
}
