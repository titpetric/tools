package components

import (
	"strings"
	"testing"
)

// TestGoVersion checks a module below the highest go directive the workspace
// declares is coloured amber, and that a module declaring none writes no cell
// rather than an empty one.
func TestGoVersion(t *testing.T) {
	if got := GoVersion("", false); got != nil {
		t.Errorf("GoVersion(\"\") = %q, want no cell", got)
	}

	current := GoVersion("1.27.0", false)
	if !strings.Contains(current.Line(0), ColorTeal) || !strings.Contains(current.Line(0), "1.27.0") {
		t.Errorf("GoVersion(1.27.0, false) = %q, want a teal 1.27.0", current)
	}
	if outdated := GoVersion("1.25", true); !strings.Contains(outdated.Line(0), ColorAmber) {
		t.Errorf("GoVersion(1.25, true) = %q, want amber", outdated)
	}
}
