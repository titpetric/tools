package main

import (
	"bytes"
	"os"
	"testing"
)

// TestSupportsANSI covers what decides between the drawn tables and markdown.
// A character device that is not a terminal is the case the mode bits get
// wrong: /dev/null carries os.ModeCharDevice and reads no escape code.
func TestSupportsANSI(t *testing.T) {
	if supportsANSI(&bytes.Buffer{}) {
		t.Error("supportsANSI() called a buffer a terminal")
	}

	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer null.Close()
	if supportsANSI(null) {
		t.Errorf("supportsANSI(%s) = true, want false", os.DevNull)
	}

	file, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer file.Close()
	if supportsANSI(file) {
		t.Error("supportsANSI() called a regular file a terminal")
	}

	// TERM=dumb is a terminal that was asked not to be written to in colour.
	t.Setenv("TERM", "dumb")
	if supportsANSI(os.Stdout) {
		t.Error("supportsANSI() wrote colour to TERM=dumb")
	}
}

// TestTerminalWidth checks that anything without a terminal behind it reports
// no width, which is what leaves a cell unfolded.
func TestTerminalWidth(t *testing.T) {
	if got := terminalWidth(&bytes.Buffer{}); got != 0 {
		t.Errorf("terminalWidth(buffer) = %d, want 0", got)
	}

	file, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer file.Close()
	if got := terminalWidth(file); got != 0 {
		t.Errorf("terminalWidth(file) = %d, want 0", got)
	}
}

func TestCellWidth(t *testing.T) {
	tests := []struct {
		name     string
		terminal int
		widths   []int
		want     int
	}{
		// Two leading columns of 7 and 23 cost 1 + (7+3) + (23+3) = 37, and
		// the border closing the row costs one more.
		{name: "room to fold into", terminal: 100, widths: []int{7, 23, 0}, want: 62},
		// A terminal with no width is one there is nothing to measure, and a
		// cell with fewer than twenty columns left is not worth folding.
		{name: "no terminal", terminal: 0, widths: []int{7, 23, 0}},
		{name: "unmeasurable", terminal: -1, widths: []int{7, 23, 0}},
		{name: "too narrow", terminal: 50, widths: []int{7, 23, 0}},
	}

	for _, test := range tests {
		if got := cellWidth(test.terminal, test.widths); got != test.want {
			t.Errorf("%s: cellWidth(%d, %v) = %d, want %d", test.name, test.terminal, test.widths, got, test.want)
		}
	}
}
