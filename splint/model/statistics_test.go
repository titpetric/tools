package model

import "testing"

func TestNewStatistics(t *testing.T) {
	labels := []string{"Package", "Files"}
	rows := [][]string{{"./model", "12"}, {"./render", "5"}}

	got := NewStatistics(labels, rows)
	if len(got.Labels) != 2 || len(got.Rows) != 2 {
		t.Fatalf("NewStatistics() = %#v", got)
	}
	if got.Header != "" || got.Footer != "" {
		t.Errorf("NewStatistics() invented a header or a footer: %#v", got)
	}
	if got.Empty() {
		t.Error("Empty() said a table with rows is empty")
	}

	// A table with no rows is a linter that had nothing to count, which a
	// renderer leaves out rather than drawing an empty box.
	if !NewStatistics(labels, nil).Empty() {
		t.Error("Empty() said a table with no rows is not empty")
	}
}

// TestStatisticsOptions covers both options, since the header prints above the
// table and the footer below it and a caller may want either.
func TestStatisticsOptions(t *testing.T) {
	got := NewStatistics(
		[]string{"Package"},
		[][]string{{"./model"}},
		HeaderText("What this table is."),
		FooterText("One line of summary."),
	)

	if got.Header != "What this table is." {
		t.Errorf("Header = %q", got.Header)
	}
	if got.Footer != "One line of summary." {
		t.Errorf("Footer = %q", got.Footer)
	}
}
