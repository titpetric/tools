package components

import (
	"testing"
)

// TestCell covers the measurements a table takes of a cell. Width is the
// visual width, so the colour codes a line carries do not count towards it,
// and an empty cell is still one line tall: a row of nothing is still a row.
func TestCell(t *testing.T) {
	cell := Cell{"one", ColorGreen + "three" + ColorReset, "two"}

	if got, want := cell.Width(), 5; got != want {
		t.Errorf("Cell.Width() = %d, want %d", got, want)
	}
	if got, want := cell.Height(), 3; got != want {
		t.Errorf("Cell.Height() = %d, want %d", got, want)
	}
	if cell.Empty() {
		t.Error("Cell.Empty() = true for a cell holding three lines")
	}

	var empty Cell
	if got, want := empty.Width(), 0; got != want {
		t.Errorf("Cell.Width() of an empty cell = %d, want %d", got, want)
	}
	if got, want := empty.Height(), 1; got != want {
		t.Errorf("Cell.Height() of an empty cell = %d, want %d", got, want)
	}
	if !empty.Empty() {
		t.Error("Cell.Empty() = false for a cell holding nothing")
	}
}

// TestDivider checks the divider is one line of nothing but the sentinel a
// renderer turns into a rule, so a caller never writes the sentinel itself.
func TestDivider(t *testing.T) {
	got := Divider()

	if got.Height() != 1 {
		t.Fatalf("Divider() = %d lines, want 1: %q", got.Height(), got)
	}
	if got.Line(0) != Separator {
		t.Errorf("Divider() = %q, want the separator sentinel", got.Line(0))
	}
	if got.Empty() {
		t.Error("Divider() reads as an empty cell")
	}
}

// TestCell_Line checks a line past the end of a cell reads as empty, which is
// what lets a short cell sit beside a tall one without the caller counting.
func TestCell_Line(t *testing.T) {
	cell := Cell{"first", "second"}
	for i, want := range map[int]string{0: "first", 1: "second", 2: "", 7: ""} {
		if got := cell.Line(i); got != want {
			t.Errorf("Cell.Line(%d) = %q, want %q", i, got, want)
		}
	}
}

// TestRows_RowHeight checks a row is as tall as its tallest cell, and one line
// tall at the least.
func TestRows_RowHeight(t *testing.T) {
	tests := []struct {
		name string
		in   Rows
		want int
	}{
		{name: "no cells", in: nil, want: 1},
		{name: "every cell empty", in: Rows{nil, nil}, want: 1},
		{name: "the tallest cell wins", in: Rows{Cell{"a"}, Cell{"a", "b", "c"}, Cell{"a", "b"}}, want: 3},
	}
	for _, test := range tests {
		if got := test.in.RowHeight(); got != test.want {
			t.Errorf("%s: Rows.RowHeight() = %d, want %d", test.name, got, test.want)
		}
	}
}
