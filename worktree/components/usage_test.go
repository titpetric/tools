package components

import (
	"strings"
	"testing"
)

// TestUsage checks a module nothing uses and that uses nothing writes no cell
// at all, in either form.
func TestUsage(t *testing.T) {
	var none Usage
	if got := none.Compact(); got != nil {
		t.Errorf("Usage{}.Compact() = %q, want no cell", got)
	}
	if got := none.Verbose(); got != nil {
		t.Errorf("Usage{}.Verbose() = %q, want no cell", got)
	}
}

// TestUsage_Compact checks the compact cell is one line of two counts, and
// that one dependent being behind colours the count even when the rest are
// current: the column is there to be scanned for what needs moving.
func TestUsage_Compact(t *testing.T) {
	u := Usage{
		UsedBy: []Dependent{{Name: "alpha"}, {Name: "beta", Outdated: true}},
		Uses:   []string{"gamma"},
	}

	got := u.Compact()
	if got.Height() != 1 {
		t.Fatalf("Usage.Compact() = %d lines, want 1: %q", got.Height(), got)
	}
	line := got.Line(0)
	if !strings.Contains(line, "↑ ") || !strings.Contains(line, "↓ ") {
		t.Errorf("Usage.Compact() = %q, want both directions", line)
	}
	if !strings.Contains(line, ColorYellow+"2") {
		t.Errorf("Usage.Compact() = %q, want the used-by count coloured for the outdated one", line)
	}
	if !strings.Contains(line, ColorWhite+"1") {
		t.Errorf("Usage.Compact() = %q, want a uses count of 1", line)
	}
}

// TestUsage_Verbose checks the verbose cell names every dependency, one line
// per direction, and colours each dependent by whether it is behind.
func TestUsage_Verbose(t *testing.T) {
	u := Usage{
		UsedBy: []Dependent{{Name: "alpha"}, {Name: "beta", Outdated: true}},
		Uses:   []string{"gamma", "delta"},
	}

	got := u.Verbose()
	if got.Height() != 2 {
		t.Fatalf("Usage.Verbose() = %d lines, want 2: %q", got.Height(), got)
	}
	if !strings.Contains(got.Line(0), ColorGreen+"alpha") || !strings.Contains(got.Line(0), ColorYellow+"beta") {
		t.Errorf("Usage.Verbose() = %q, want alpha current and beta behind", got.Line(0))
	}
	for _, name := range []string{"gamma", "delta"} {
		if !strings.Contains(got.Line(1), name) {
			t.Errorf("Usage.Verbose() = %q, missing %q", got.Line(1), name)
		}
	}

	// One direction alone is one line, not a line and a blank.
	if got := (Usage{Uses: []string{"gamma"}}).Verbose(); got.Height() != 1 {
		t.Errorf("Usage.Verbose() of uses alone = %d lines, want 1: %q", got.Height(), got)
	}
}
