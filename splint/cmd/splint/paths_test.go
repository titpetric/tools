package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/titpetric/tools/splint/loader"
)

// tiny is a module of one package, written where a test asks for it. The
// fixture module is read from where it sits and never written to; a test about
// where a document lands needs a tree it owns.
const tinyModule = `module example.com/tiny

go 1.27.0
`

// tinySource is one documented function, on lines a coverage profile can name.
const tinySource = `// Package tiny is one package with one function in it.
package tiny

// Do returns one.
func Do() int {
	return 1
}
`

// writeTree writes the tiny module into a new directory under root and returns
// the path to it.
func writeTree(t *testing.T, root, name string) string {
	t.Helper()

	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for file, content := range map[string]string{
		"go.mod":  tinyModule,
		"tiny.go": tinySource,
	} {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// runIn runs the command with the process in dir, the way a shell would run it
// there, and returns what it wrote.
func runIn(t *testing.T, dir string, args ...string) string {
	t.Helper()

	t.Chdir(dir)

	var out bytes.Buffer
	if _, err := run(context.Background(), args, &out); err != nil {
		t.Fatalf("run(%v) error = %v", args, err)
	}
	return out.String()
}

// TestSaveWithASourcePath covers -save and -i together. The parse moves the
// process into the tree it reads, and the output path was joined onto the
// source path a second time on the way back out: "-i tree -save ." wrote
// tree/tree/splint.json, and the directory it named does not exist.
func TestSaveWithASourcePath(t *testing.T) {
	root := t.TempDir()
	tree := writeTree(t, root, "tree")

	runIn(t, root, "-i", "tree", "-save", ".")

	if _, err := os.Stat(filepath.Join(tree, saveFile)); err != nil {
		t.Errorf("-save wrote no %s beside the tree: %v", saveFile, err)
	}
	if _, err := os.Stat(filepath.Join(tree, "tree")); err == nil {
		t.Error("-save resolved the source path twice")
	}
}

// TestOutputWithASourcePath covers the other flag naming a file: -output is
// relative to the directory the command was run in, whichever tree it reads.
func TestOutputWithASourcePath(t *testing.T) {
	root := t.TempDir()
	tree := writeTree(t, root, "tree")

	runIn(t, root, "-i", "tree", "-output", "doc.json", ".")

	if _, err := os.Stat(filepath.Join(root, "doc.json")); err != nil {
		t.Errorf("-output wrote nothing where it was told: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tree, "doc.json")); err == nil {
		t.Error("-output wrote into the tree it read rather than into the working directory")
	}

	// The same path names the same file on the way back in.
	doc, err := loader.Load(filepath.Join(root, "doc.json"))
	if err != nil {
		t.Fatalf("the written document does not load: %v", err)
	}
	if len(doc.Packages) != 1 {
		t.Errorf("the written document holds %d packages, want 1", len(doc.Packages))
	}

	runIn(t, root, "-input", "doc.json")
}
