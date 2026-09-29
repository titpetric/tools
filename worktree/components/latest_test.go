package components

import (
	"strings"
	"testing"
)

// TestLatest checks a repository with no release tag writes no cell, rather
// than a cell holding nothing.
func TestLatest(t *testing.T) {
	if got := Latest(""); got != nil {
		t.Errorf("Latest(\"\") = %q, want no cell", got)
	}
	if got := Latest("v0.6.0"); !strings.Contains(got.Line(0), "v0.6.0") {
		t.Errorf("Latest(v0.6.0) = %q, want the tag in it", got)
	}
}
