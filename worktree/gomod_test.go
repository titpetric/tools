package main

import (
	"os"
	"path/filepath"
	"reflect"
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

// TestDiff reads the two go.mod fixtures and pins every case the comparison
// has to tell apart, which is what the pair of them is written to hold: a
// requirement added, one dropped, one moved to another version, one that held,
// and the indirect ones that are left out of the report.
func TestDiff(t *testing.T) {
	before, after := testdataRequires(t, "go-before.mod"), testdataRequires(t, "go-after.mod")

	want := []DiffResult{
		{Path: "github.com/google/go-cmp", Change: fieldAdded, New: "v0.7.0"},
		{Path: "golang.org/x/mod", Change: fieldChanged, Old: "v0.40.0", New: "v0.41.0"},
		// Indirect before and direct after, so it is reported: a requirement
		// direct on either side is one the module decided on.
		{Path: "golang.org/x/sync", Change: fieldChanged, Old: "v0.22.0", New: "v0.23.0"},
		{Path: "gopkg.in/yaml.v3", Change: fieldRemoved, Old: "v3.0.1"},
	}
	if got := Diff(before, after); !reflect.DeepEqual(got, want) {
		t.Errorf("Diff() = %+v, want %+v", got, want)
	}

	// github.com/charmbracelet/x/ansi holds its version across both files, and
	// runewidth moved while staying indirect, so neither is in the report.
	for _, change := range Diff(before, after) {
		for _, unwanted := range []string{"github.com/charmbracelet/x/ansi", "github.com/mattn/go-runewidth", "github.com/rivo/uniseg"} {
			if change.Path == unwanted {
				t.Errorf("Diff() reported %s, which did not move a consumer can see", unwanted)
			}
		}
	}

	if got := Diff(after, after); len(got) != 0 {
		t.Errorf("Diff() of one revision against itself = %+v, want nothing", got)
	}
}

// TestDiffResultVersion covers the version cell of each category, which is the
// one place a change reads as two versions rather than one.
func TestDiffResultVersion(t *testing.T) {
	tests := map[DiffResult]string{
		{Change: fieldAdded, New: "v1.1.0"}:                  "v1.1.0",
		{Change: fieldChanged, Old: "v1.0.0", New: "v1.1.0"}: "v1.0.0 -> v1.1.0",
		{Change: fieldRemoved, Old: "v1.0.0"}:                "v1.0.0",
	}
	for change, want := range tests {
		if got := change.Version(); got != want {
			t.Errorf("DiffResult{%s}.Version() = %q, want %q", change.Change, got, want)
		}
	}
}

// testdataRequires reads the requirements of one of the go.mod fixtures.
func testdataRequires(t *testing.T, name string) []requireInfo {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	reqs, err := parseRequires(data)
	if err != nil {
		t.Fatalf("parseRequires(%s) error: %v", name, err)
	}
	return reqs
}
