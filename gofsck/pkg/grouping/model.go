package grouping

// Report contains grouping analysis results.
type Report struct {
	Total      int         `json:"total"`
	Passing    int         `json:"passing"`
	Violations []Violation `json:"violations,omitempty"`
}

// Violation represents a symbol that doesn't match its expected filename.
type Violation struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Symbol  string `json:"symbol"`
	Message string `json:"message"`
}
