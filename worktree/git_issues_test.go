package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/titpetric/tools/worktree/components"
)

// TestParseGHIssueList covers what the gh output is read into: the number
// becomes the id it is written as, and a timestamp is cut back to the day,
// which is all the column has room for.
func TestParseGHIssueList(t *testing.T) {
	data := []byte(`[
		{"number":18,"title":"telemetry views are unreadable on mobile","createdAt":"2026-03-14T09:21:07Z"},
		{"number":4,"title":"no date at all","createdAt":""}
	]`)

	want := []components.Issue{
		{ID: "#18", Title: "telemetry views are unreadable on mobile", Date: "2026-03-14"},
		{ID: "#4", Title: "no date at all", Date: ""},
	}
	if got := parseGHIssueList(data); !reflect.DeepEqual(got, want) {
		t.Errorf("parseGHIssueList() = %+v, want %+v", got, want)
	}

	// Output that is not a list of issues is no issues, not a failed run: gh
	// is asked and its answer is taken or left.
	for _, data := range []string{"", "[]", "not json", `{"number":1}`} {
		if got := parseGHIssueList([]byte(data)); len(got) != 0 {
			t.Errorf("parseGHIssueList(%q) = %+v, want nothing", data, got)
		}
	}
}

// TestGHIssueCachePath checks the cache path is derived from the directory, so
// two checkouts do not read each other's issues, and that the same directory
// always names the same file.
func TestGHIssueCachePath(t *testing.T) {
	root := t.TempDir()
	one, two := filepath.Join(root, "one"), filepath.Join(root, "two")

	first := ghIssueCachePath(one)
	if first != ghIssueCachePath(one) {
		t.Error("ghIssueCachePath() named two files for one directory")
	}
	if first == ghIssueCachePath(two) {
		t.Errorf("ghIssueCachePath() named one file for two directories: %s", first)
	}
	if !strings.Contains(filepath.Base(first), "worktree-gh-issues-") {
		t.Errorf("ghIssueCachePath() = %q, want a worktree-gh-issues- name", first)
	}
}
