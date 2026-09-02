package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/loader"
	"github.com/titpetric/tools/splint/model"
)

// tinyProfile covers the body of Do and nothing else. The line and column
// range is the block the toolchain records for the body of the function
// declared on line 5 of tiny.go.
const tinyProfile = `mode: set
example.com/tiny/tiny.go:5.16,7.2 1 1
`

// TestAppendCoverage covers the overlay end to end: the profile is read during
// the parse, and the document carries what it said.
func TestAppendCoverage(t *testing.T) {
	root := t.TempDir()
	tree := writeTree(t, root, "tree")

	if err := os.WriteFile(filepath.Join(root, "pkg.cov"), []byte(tinyProfile), 0o644); err != nil {
		t.Fatal(err)
	}

	runIn(t, root, "-i", "tree", "-save", "--append-coverage=pkg.cov", ".")

	doc, err := loader.Load(filepath.Join(tree, saveFile))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Packages) != 1 {
		t.Fatalf("the document holds %d packages, want 1", len(doc.Packages))
	}

	def := doc.Packages[0]
	if def.Package.Complexity == nil || def.Package.Complexity.Coverage != 100 {
		t.Errorf("package coverage = %v, want 100", def.Package.Complexity)
	}

	fn := def.Funcs.Find(func(d *model.Declaration) bool { return d.Name == "Do" })
	if fn == nil {
		t.Fatal("the document holds no Do")
	}
	if fn.Complexity == nil || fn.Complexity.Coverage != 100 {
		t.Errorf("Do coverage = %v, want 100", fn.Complexity)
	}
}

// TestAppendCoverageWithInput covers the combination that is refused: the
// overlay reads the line ranges a parse recorded, and -input is the flag that
// skips the parse.
func TestAppendCoverageWithInput(t *testing.T) {
	var out bytes.Buffer
	_, err := run(context.Background(), []string{"--append-coverage=pkg.cov", "-input", "doc.json"}, &out)
	if err == nil {
		t.Fatal("run() accepted --append-coverage with -input")
	}
	if !strings.Contains(err.Error(), "append-coverage") || !strings.Contains(err.Error(), "input") {
		t.Errorf("run() error = %v, want it to name both flags", err)
	}
}

// TestAppendCoverageMissingProfile covers the profile that is not there, which
// is a failed run rather than a document with no coverage in it.
func TestAppendCoverageMissingProfile(t *testing.T) {
	var out bytes.Buffer
	_, err := run(context.Background(), []string{"-i", fixture, "--append-coverage=nowhere.cov", "./..."}, &out)
	if err == nil {
		t.Fatal("run() accepted a profile that does not exist")
	}
	if !strings.Contains(err.Error(), "nowhere.cov") {
		t.Errorf("run() error = %v, want it to name the file", err)
	}
}
