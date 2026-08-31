package model

import "testing"

func TestIsExported(t *testing.T) {
	tests := map[string]bool{
		"Declaration": true,
		"declaration": false,
		"ID":          true,
		"_private":    false,
		"":            false,
		"Ünicode":     true,
		"ünicode":     false,
	}

	for name, want := range tests {
		if got := isExported(name); got != want {
			t.Errorf("isExported(%q) = %v, want %v", name, got, want)
		}
	}
}
