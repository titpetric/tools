package model

type Complexity struct {
	Cognitive  int `json:"Cognitive" yaml:"Cognitive"`
	Cyclomatic int `json:"Cyclomatic" yaml:"Cyclomatic"`
	Lines      int `json:"Lines" yaml:"Lines"`

	// Coverage is filled out of band (summary coverfunc).
	Coverage float64 `json:"Coverage,omitempty" yaml:"Coverage,omitempty"`
}
