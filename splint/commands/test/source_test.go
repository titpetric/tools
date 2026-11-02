package test

import (
	"context"
	"testing"
)

// TestRelevant covers the paths a run reads: Go source of this tree, and no
// fixture, vendor or hidden file.
func TestRelevant(t *testing.T) {
	for _, one := range []struct {
		path string
		want bool
	}{
		{"p.go", true},
		{"p_test.go", true},
		{"pkg/p.go", true},
		{"vendor/p.go", false},
		{"pkg/testdata/p.go", false},
		{"pkg/.hidden/p.go", false},
		{"pkg/_build/p.go", false},
		{"p.txt", false},
	} {
		if got := relevant(one.path); got != one.want {
			t.Errorf("relevant(%q) = %v, want %v", one.path, got, one.want)
		}
	}
}

// TestChangesOfACleanTree covers a run before anything moved: the revision and
// the working tree are the same, so no path changed.
func TestChangesOfACleanTree(t *testing.T) {
	dir := repo(t)
	src, err := newSource(context.Background(), Options{SourcePath: dir, Pattern: "./..."})
	if err != nil {
		t.Fatalf("newSource() error = %v", err)
	}

	changes, err := src.changes(context.Background())
	if err != nil {
		t.Fatalf("changes() error = %v", err)
	}
	if len(changes) != 0 {
		t.Fatalf("a clean tree changed %v", changes)
	}
}
