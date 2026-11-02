package test

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestSubfolderSeesOnlyItsChanges covers the run in a folder: the change
// beside it is read and the change above it is not.
func TestSubfolderSeesOnlyItsChanges(t *testing.T) {
	dir := repo(t)
	write(t, dir, "pkg/p.go", "package pkg\n\nfunc P() int { return 1 }\n")
	write(t, dir, "pkg/p_test.go", "package pkg\n\nimport \"testing\"\n\nfunc TestP(t *testing.T) { P() }\n")
	write(t, dir, "other/o.go", "package other\n\nfunc O() int { return 1 }\n")
	write(t, dir, "other/o_test.go", "package other\n\nimport \"testing\"\n\nfunc TestO(t *testing.T) { O() }\n")
	commit(t, dir, "tree")
	write(t, dir, "pkg/p.go", "package pkg\n\nfunc P() int { return 2 }\n")
	write(t, dir, "other/o.go", "package other\n\nfunc O() int { return 2 }\n")

	got := list(t, Options{SourcePath: filepath.Join(dir, "pkg"), Pattern: "."})
	if strings.TrimSpace(got) != "TestP" {
		t.Fatalf("a run in a subfolder selected %q, want TestP", got)
	}
}
