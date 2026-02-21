package wraphandler

// Report contains wraphandler analysis results.
type Report struct {
	Total      int         `json:"total"`
	Passing    int         `json:"passing"`
	Violations []Violation `json:"violations,omitempty"`
}

// Violation represents a handler without a corresponding unexported wrapper.
type Violation struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Symbol   string `json:"symbol"`
	Receiver string `json:"receiver,omitempty"`
	Message  string `json:"message"`
}
