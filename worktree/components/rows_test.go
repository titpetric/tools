package components

import (
	"testing"
)

// TestRowsIsARowOfCells checks the row type carries the cells it is built
// from, which is what the table walks column by column.
func TestRowsIsARowOfCells(t *testing.T) {
	row := Rows{Cell{"path"}, nil, Cell{"one", "two"}}

	if got, want := len(row), 3; got != want {
		t.Fatalf("len(Rows) = %d, want %d", got, want)
	}
	if !row[1].Empty() {
		t.Error("Rows kept content in a cell that was given none")
	}
	if got, want := row[2].Line(1), "two"; got != want {
		t.Errorf("Rows[2].Line(1) = %q, want %q", got, want)
	}
}
