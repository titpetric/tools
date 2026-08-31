package model

// Statistics is one table a linter reports.
//
// It is a rendering rather than a measurement: the numbers themselves are in
// LintMetrics, and this is how they read. A linter decides its own columns,
// because what is worth a column is what the linter measured.
type Statistics struct {
	// Labels are the column headings.
	Labels []string `json:"Labels" yaml:"Labels"`

	// Rows are the cells, one slice per row, in label order.
	Rows [][]string `json:"Rows" yaml:"Rows"`

	// Header prints above the table and Footer below it. The footer is where
	// a linter writes its one line of summary, which is the line a reader
	// takes away when they read nothing else.
	Header string `json:"Header,omitempty" yaml:"Header,omitempty"`
	Footer string `json:"Footer,omitempty" yaml:"Footer,omitempty"`
}

// StatisticsOption is what a caller passes to say more than the table itself.
type StatisticsOption func(*Statistics)

// HeaderText prints above the table.
func HeaderText(text string) StatisticsOption {
	return func(s *Statistics) { s.Header = text }
}

// FooterText prints below the table.
func FooterText(text string) StatisticsOption {
	return func(s *Statistics) { s.Footer = text }
}

// NewStatistics returns one table.
func NewStatistics(labels []string, rows [][]string, options ...StatisticsOption) Statistics {
	out := Statistics{Labels: labels, Rows: rows}
	for _, option := range options {
		option(&out)
	}
	return out
}

// Empty reports a table with no rows, which is a linter that had nothing to
// count rather than one that counted nothing.
func (s Statistics) Empty() bool {
	return len(s.Rows) == 0
}
