package main

import (
	"testing"
)

// TestRequireInfo covers the one thing a requirement carries beyond its path
// and version: whether the module imports it itself. The mark decides whether a
// release reports the requirement at all, so its zero value has to be direct.
func TestRequireInfo(t *testing.T) {
	direct := requireInfo{path: "golang.org/x/mod", version: "v0.41.0"}
	if direct.indirect {
		t.Error("requireInfo{} is indirect by default, want direct")
	}

	before := []requireInfo{direct, {path: "golang.org/x/sync", version: "v0.22.0", indirect: true}}
	after := []requireInfo{
		{path: "golang.org/x/mod", version: "v0.41.0"},
		{path: "golang.org/x/sync", version: "v0.23.0", indirect: true},
	}
	if got := Diff(before, after); len(got) != 0 {
		t.Errorf("Diff() reported %+v, want nothing: the direct one held and the indirect one is not reported", got)
	}
}

// TestGitStatus checks the counts a module's git state is read into, which is
// what the workspace table colours a row on.
func TestGitStatus(t *testing.T) {
	var clean gitStatus
	if clean.Unpushed != 0 || clean.Modified != 0 || len(clean.DiffLines) != 0 {
		t.Errorf("gitStatus{} = %+v, want a clean checkout", clean)
	}

	dirty := gitStatus{Unpushed: 2, Modified: 1, DiffLines: []string{"verdict.go +12/-3"}}
	if dirty.Unpushed != 2 || dirty.Modified != 1 || len(dirty.DiffLines) != 1 {
		t.Errorf("gitStatus = %+v, want the counts it was given", dirty)
	}
}

// TestVersionRefs and TestLatestTags cover the two lookup tables the workspace
// scan builds: what each module requires of every other, and the newest tag
// each one carries.
func TestVersionRefs(t *testing.T) {
	refs := versionRefs{
		"example.com/app": {"example.com/lib": "v0.1.0"},
	}
	if got := refs["example.com/app"]["example.com/lib"]; got != "v0.1.0" {
		t.Errorf("versionRefs lookup = %q, want %q", got, "v0.1.0")
	}
	// A module nothing is known about reads as no requirements, not a panic.
	if got := refs["example.com/missing"]["example.com/lib"]; got != "" {
		t.Errorf("versionRefs lookup of an absent module = %q, want %q", got, "")
	}
}

func TestLatestTags(t *testing.T) {
	tags := latestTags{"example.com/lib": "v0.2.0"}
	if got, want := tags["example.com/lib"], "v0.2.0"; got != want {
		t.Errorf("latestTags lookup = %q, want %q", got, want)
	}
	if got := tags["example.com/unreleased"]; got != "" {
		t.Errorf("latestTags lookup of an unreleased module = %q, want %q", got, "")
	}
}
