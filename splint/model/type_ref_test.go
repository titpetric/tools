package model

import "testing"

// TestTypeRef covers the trims a reference goes through: variadics, slices,
// maps, pointers and generic instantiations.
func TestTypeRef(t *testing.T) {
	tests := map[string]string{
		"Client":         "Client",
		"*Client":        "Client",
		"...Client":      "Client",
		"[]Client":       "Client",
		"map[string]int": "int",
		"Stack[T]":       "Stack",
		"*Stack[T]":      "Stack",
		"*Cache[K, V]":   "Cache",
	}

	for in, want := range tests {
		if got := TypeRef(in); got != want {
			t.Errorf("TypeRef(%q) = %q, want %q", in, got, want)
		}
	}
}
