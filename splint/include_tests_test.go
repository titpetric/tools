package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/loader"
)

// TestWrittenDocumentHasNoTestPackages covers --include-tests. The parse reads
// the test files whether or not it was asked to, because the pairing and
// coverage linters have nothing to check without them; the document a run
// writes carries them only when the flag asked for it.
func TestWrittenDocumentHasNoTestPackages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.json")
	runArgs(t, "-i", fixture, "--output", path, "./...")

	doc, err := loader.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, def := range doc.Packages {
		if def.TestPackage {
			t.Errorf("the written document holds the test package %s", def.ImportPath)
		}
		for _, decl := range def.DeclarationList() {
			if decl.IsTestScope() {
				t.Errorf("the written document holds %s, declared in %s", decl.Symbol(), decl.File)
			}
		}
		for _, file := range def.Files {
			if file.Test {
				t.Errorf("the written document holds the test file %s", file.Name)
			}
		}
	}

	// The linters still read them: the fixture has a file with no test beside
	// it, which is a finding only when the tests were read.
	found, _ := runArgs(t, "-i", fixture, "-linters", "pairing", "./...")
	if !strings.Contains(found, "pairing/unpaired") {
		t.Error("the pairing linter found nothing, so the parse did not read the tests")
	}
}

// TestWrittenDocumentKeepsTestPackages covers the flag being given.
func TestWrittenDocumentKeepsTestPackages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.json")
	runArgs(t, "-i", fixture, "-include-tests", "--output", path, "./...")

	doc, err := loader.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	var tests int
	for _, def := range doc.Packages {
		if def.TestPackage {
			tests++
		}
	}
	if tests == 0 {
		t.Error("-include-tests wrote a document with no test package in it")
	}
}
