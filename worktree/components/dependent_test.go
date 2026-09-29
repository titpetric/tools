package components

import (
	"strings"
	"testing"
)

// TestDependent checks the two things a dependent carries: the name a usage
// cell writes, and whether the version it requires is behind, which is what
// decides the colour it is written in.
func TestDependent(t *testing.T) {
	current := Dependent{Name: "titpetric/tools"}
	if current.Outdated {
		t.Error("Dependent{} is outdated by default")
	}

	behind := Dependent{Name: "titpetric/tools", Outdated: true}
	if got := (Usage{UsedBy: []Dependent{behind}}).Compact(); !strings.Contains(got.Line(0), ColorYellow) {
		t.Errorf("one outdated dependent did not colour the count: %q", got)
	}
	if got := (Usage{UsedBy: []Dependent{current}}).Compact(); !strings.Contains(got.Line(0), ColorGreen) {
		t.Errorf("an up to date dependent did not colour the count green: %q", got)
	}
}
