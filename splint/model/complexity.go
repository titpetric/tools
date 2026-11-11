package model

// Complexity is how much a declaration branches, how long it runs and how it exits.
type Complexity struct {
	Cognitive  int `json:"Cognitive" yaml:"Cognitive"`
	Cyclomatic int `json:"Cyclomatic" yaml:"Cyclomatic"`
	Lines      int `json:"Lines" yaml:"Lines"`

	// Exits is how many ways a function leaves: each return, each panic,
	// os.Exit and log.Fatal call, plus the end of a body that runs off its
	// closing brace. It holds zero for anything but a function.
	Exits int `json:"Exits,omitempty" yaml:"Exits,omitempty"`

	// Blocks is how many braces a function body opens inside itself: each
	// block of an if, a loop, a switch or a select, each function literal,
	// and each composite or struct literal written out. It holds zero for a
	// body that branches nowhere, and for anything but a function.
	Blocks int `json:"Blocks,omitempty" yaml:"Blocks,omitempty"`

	// Coverage is the share of the statements that ran, from the Go coverage
	// profile --append-coverage names. It holds zero for a parse with no
	// profile, and for a declaration holding no statements to measure.
	Coverage float64 `json:"Coverage,omitempty" yaml:"Coverage,omitempty"`
}
