package prose

// Metric is the shape of a tree's prose, as one run measured it.
type Metric struct {
	// Blocks and Words tally the prose behind the rest.
	Blocks int `json:"Blocks" yaml:"Blocks"`
	Words  int `json:"Words" yaml:"Words"`

	// BlockWords is the median comment block, in words. A block covers one
	// symbol. This answers how far a function's comment runs.
	BlockWords int `json:"BlockWords" yaml:"BlockWords"`

	// SentenceWords is the median sentence and P90Words the ninetieth
	// percentile, where the long ones live.
	SentenceWords int `json:"SentenceWords" yaml:"SentenceWords"`
	P90Words      int `json:"P90Words" yaml:"P90Words"`

	// Subordinators counts per thousand words, and Stacked is the share of
	// blocks holding two or more.
	Subordinators float64 `json:"Subordinators" yaml:"Subordinators"`
	Stacked       float64 `json:"Stacked" yaml:"Stacked"`

	// Shapes is how many constructions the run reported.
	Shapes int `json:"Shapes" yaml:"Shapes"`
}
