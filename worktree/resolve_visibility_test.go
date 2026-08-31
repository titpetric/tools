package main

import (
	"os/exec"
	"testing"
)

func TestReadVisibilitySkipsWhatItCannotCount(t *testing.T) {
	got := readVisibility(t.TempDir())
	if got.Skipped != "not a go module" {
		t.Errorf("readVisibility() skipped with %q, want %q", got.Skipped, "not a go module")
	}
	if len(got.Packages) > 0 {
		t.Errorf("readVisibility() counted %d packages of a directory holding none", len(got.Packages))
	}
}

func TestReadVisibilityCountsTheModuleItIsPointedAt(t *testing.T) {
	if _, err := exec.LookPath("gofsck"); err != nil {
		t.Skip("gofsck is not installed")
	}

	got := readVisibility(".")
	if got.Skipped != "" {
		t.Fatalf("readVisibility() skipped: %s", got.Skipped)
	}

	// This module is one package and a components package under it, and the
	// command itself is left out of the count.
	var found bool
	for _, pkg := range got.Packages {
		if pkg.Package != "./components" {
			continue
		}
		found = true
		if pkg.ExportedTypes+pkg.ExportedFuncs == 0 {
			t.Errorf("readVisibility() counted nothing exported in %s", pkg.Package)
		}
	}
	if !found {
		t.Errorf("readVisibility() did not count ./components: %#v", got.Packages)
	}
}
