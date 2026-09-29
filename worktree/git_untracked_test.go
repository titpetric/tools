package main

import (
	"reflect"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestCollapseUntrackedFoldsASubtree checks that the files of a directory
// holding nothing tracked are summed into the one entry standing in for them,
// while a file git names on its own is carried over as it is.
func TestCollapseUntrackedFoldsASubtree(t *testing.T) {
	files := []untrackedEntry{
		{Path: "demos/common/src/Auth.php", Files: 1, Lines: 117},
		{Path: "demos/common/src/Log.php", Files: 1, Lines: 95},
		{Path: "demos/common/src/Mock/Memory.php", Files: 1, Lines: 284},
		{Path: "demos/common/README.md", Files: 1, Lines: 4},
		{Path: "main_test.go", Files: 1, Lines: 61},
	}
	collapsed := []string{"demos/common/", "main_test.go"}

	want := []untrackedEntry{
		{Path: "demos/common/", Dirs: 2, Files: 4, Lines: 500},
		{Path: "main_test.go", Files: 1, Lines: 61},
	}

	got := collapseUntracked(collapsed, files)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collapseUntracked() = %#v, want %#v", got, want)
	}
}

// TestCollapseUntrackedKeepsAnUnlistedFile checks that a collapsed path the
// full listing does not hold still produces an entry, rather than being
// dropped from the output.
func TestCollapseUntrackedKeepsAnUnlistedFile(t *testing.T) {
	got := collapseUntracked([]string{"PLAN.md"}, nil)
	want := []untrackedEntry{{Path: "PLAN.md", Files: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collapseUntracked() = %#v, want %#v", got, want)
	}
}

func TestFormatUntrackedEntry(t *testing.T) {
	tests := []struct {
		name  string
		entry untrackedEntry
		want  string
	}{
		{
			name:  "directory",
			entry: untrackedEntry{Path: "demos/common/", Dirs: 18, Files: 109, Lines: 7642},
			want:  "demos/common/ +18 dirs, +109 files, +7642 SLOC",
		},
		{
			name:  "flat directory",
			entry: untrackedEntry{Path: "schema/", Files: 1, Lines: 16},
			want:  "schema/ +1 file, +16 SLOC",
		},
		{
			name:  "file",
			entry: untrackedEntry{Path: "PLAN.md", Files: 1, Lines: 61},
			want:  "PLAN.md +61",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ansi.Strip(formatUntrackedEntry(test.entry))
			if got != test.want {
				t.Fatalf("formatUntrackedEntry() = %q, want %q", got, test.want)
			}
		})
	}
}
