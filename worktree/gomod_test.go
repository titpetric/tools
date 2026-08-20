package main

import (
	"path/filepath"
	"testing"
)

func TestReadGoVersion(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "module", "go.mod"), "module example.com/app\n\ngo 1.27.1\n")
	writeTestFile(t, filepath.Join(root, "no-go", "go.mod"), "module example.com/legacy\n")
	writeTestFile(t, filepath.Join(root, "broken", "go.mod"), "not a go.mod\n")

	if got, want := readGoVersion(filepath.Join(root, "module")), "1.27.1"; got != want {
		t.Fatalf("readGoVersion() = %q, want %q", got, want)
	}
	for _, dir := range []string{"no-go", "broken", "missing"} {
		if got := readGoVersion(filepath.Join(root, dir)); got != "" {
			t.Fatalf("readGoVersion(%s) = %q, want %q", dir, got, "")
		}
	}
}
