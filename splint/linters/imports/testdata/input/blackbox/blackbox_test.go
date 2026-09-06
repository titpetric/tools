package blackbox_test

import (
	"testing"
)

// TestThing writes blackbox.Thing from package blackbox_test, which is a
// package of its own and has to import the one it tests.
func TestThing(t *testing.T) {
	if blackbox.Thing() == "" {
		t.Fatal("empty")
	}
}
