package components

import (
	"strings"
	"testing"
)

// TestGit_Branch checks the branch cell: main is teal and anything else amber,
// since a checkout that is not on the default branch is worth noticing, and
// commits ahead of the remote are named beside it.
func TestGit_Branch(t *testing.T) {
	if got := (Git{}).Branch(); got != nil {
		t.Errorf("Git{}.Branch() = %q, want no cell", got)
	}

	main := Git{BranchName: "main"}.Branch()
	if !strings.Contains(main.Line(0), ColorTeal) {
		t.Errorf("Git.Branch() on main = %q, want teal", main)
	}
	if strings.Contains(main.Line(0), "ahead") {
		t.Errorf("Git.Branch() with nothing ahead = %q, want no count", main)
	}

	other := Git{BranchName: "feature", Ahead: 3}.Branch()
	if !strings.Contains(other.Line(0), ColorAmber) {
		t.Errorf("Git.Branch() off main = %q, want amber", other)
	}
	if !strings.Contains(other.Line(0), "+3 ahead") {
		t.Errorf("Git.Branch() = %q, want the commits ahead named", other)
	}
}

// TestGit_State checks the compact state cell: a clean checkout writes nothing,
// the diff stats are split into insertions and deletions, and each block is
// divided from the one above it.
func TestGit_State(t *testing.T) {
	if got := (Git{}).State(); got != nil {
		t.Errorf("Git{}.State() = %q, want no cell", got)
	}

	unpushed := Git{Unpushed: 2}.State()
	if !strings.Contains(unpushed.Line(0), "Unpushed changes: 2") {
		t.Errorf("Git.State() = %q, want the unpushed count", unpushed)
	}

	got := Git{
		Unpushed:       1,
		DiffLines:      []string{"verdict.go 12/3"},
		UntrackedFiles: []string{"testdata/go-after.mod"},
	}.State()

	lines := strings.Join(got, "\n")
	for _, want := range []string{
		"Unpushed changes: 1",
		"Local changes:",
		// The delta is split so the two halves read apart.
		"verdict.go " + ColorGreen + "12" + ColorReset + "/" + ColorRed + "3" + ColorReset,
		"Untracked files:",
		"- testdata/go-after.mod",
	} {
		if !strings.Contains(lines, want) {
			t.Errorf("Git.State() missing %q:\n%s", want, lines)
		}
	}
	if strings.Count(lines, Separator) != 2 {
		t.Errorf("Git.State() drew %d dividers between three blocks, want 2:\n%s", strings.Count(lines, Separator), lines)
	}
}

// TestGit_StateVerbose checks the verbose cell reads in the order the report
// does: the commits since the release, then what is uncommitted, then the open
// issues, with the issue columns aligned to the widest entry.
func TestGit_StateVerbose(t *testing.T) {
	if got := (Git{}).StateVerbose(); got != nil {
		t.Errorf("Git{}.StateVerbose() = %q, want no cell", got)
	}

	got := Git{
		Msgs:      []string{"e55a37a chore: update go.mod/sum dependencies", "nohashatall"},
		DiffLines: []string{"verdict.go 12/3"},
		Issues: []Issue{
			{ID: "#18", Title: "a long issue title", Date: "2026-03-14"},
			{ID: "#4", Title: "short", Date: "2026-01-02"},
		},
	}.StateVerbose()

	lines := strings.Join(got, "\n")
	for _, want := range []string{
		"Commits since release:",
		"- " + ColorTeal + "e55a37a",
		// A line with no hash in front of it is still listed, whole.
		"- " + ColorWhite + "nohashatall",
		"Local changes:",
		"Issues: 2 open",
	} {
		if !strings.Contains(lines, want) {
			t.Errorf("Git.StateVerbose() missing %q:\n%s", want, lines)
		}
	}

	order := []string{"Commits since release:", "Local changes:", "Issues: 2 open"}
	at := -1
	for _, heading := range order {
		next := strings.Index(lines, heading)
		if next <= at {
			t.Errorf("Git.StateVerbose() wrote %q out of order:\n%s", heading, lines)
		}
		at = next
	}

	// The narrower id is padded out to the wider one, so the titles line up.
	if !strings.Contains(lines, "#4 ") {
		t.Errorf("Git.StateVerbose() did not pad the issue ids:\n%s", lines)
	}
}
