package whitebox

import (
	"testing"
)

// TestBar writes whitebox.Bar from inside package whitebox, which is a name it
// already has. Importing the package it is compiled into is an import cycle,
// so nothing is added.
func TestBar(t *testing.T) {
	if whitebox.Bar() == "" {
		t.Fatal("empty")
	}
}
