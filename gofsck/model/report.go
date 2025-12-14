package model

// AnalyzerReport is the base report structure for all analyzers.
type AnalyzerReport struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"` // pairing, coverage, grouping
	Data    interface{} `json:"data"`
	Summary string      `json:"summary,omitempty"`
}

// AggregatedReport collects reports from all analyzers.
type AggregatedReport struct {
	Analyzers []*AnalyzerReport `json:"analyzers"`
	Errors    []string          `json:"errors,omitempty"`
}
