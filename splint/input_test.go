package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/loader"
	"github.com/titpetric/tools/splint/model"
)

// TestNoImplicitInput covers a splint.json sitting beside a tree: it used to
// be read instead of parsing, so a job that extracted twice got the first
// document back and the second run reported a tree nobody had read.
func TestNoImplicitInput(t *testing.T) {
	root := t.TempDir()
	tree := writeTree(t, root, "tree")

	// A document describing nothing. A run that reads it reports nothing.
	empty := model.NewDocumentRoot(tree, "astparser")
	if err := loader.Save(filepath.Join(tree, saveFile), empty); err != nil {
		t.Fatal(err)
	}

	got := runIn(t, root, "-i", "tree", "-linters", "pairing", ".")
	if !strings.Contains(got, "tiny.go") {
		t.Errorf("a run beside a %s did not read the tree:\n%s", saveFile, got)
	}
}
