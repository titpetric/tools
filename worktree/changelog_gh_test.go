package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseGHReleases checks the release list is keyed on the tag each release
// names, with the body the note is compared against.
func TestParseGHReleases(t *testing.T) {
	data := []byte(`[
		{"tag_name":"v1.0.0","name":"v1.0.0","body":"the note","draft":false},
		{"tag_name":"v0.9.0","name":"","body":"","draft":true}
	]`)

	releases, err := parseGHReleases(data)
	if err != nil {
		t.Fatalf("parseGHReleases() error = %v", err)
	}
	if len(releases) != 2 {
		t.Fatalf("parseGHReleases() read %d releases, want 2", len(releases))
	}
	if got := releases["v1.0.0"].Body; got != "the note" {
		t.Errorf("parseGHReleases() body = %q, want %q", got, "the note")
	}
	if !releases["v0.9.0"].Draft {
		t.Error("parseGHReleases() did not read the draft flag")
	}
}

// TestParseGHReleasesRejectsNonsense checks a reply that is not the list says
// so rather than reading as a repository with no releases.
func TestParseGHReleasesRejectsNonsense(t *testing.T) {
	if _, err := parseGHReleases([]byte(`{"message":"Not Found"}`)); err == nil {
		t.Fatal("parseGHReleases() accepted a reply that is not a release list")
	}
}

// TestParseGHTags checks the tag list reads as the set of names the remote
// carries.
func TestParseGHTags(t *testing.T) {
	tags, err := parseGHTags([]byte(`[{"name":"v1.0.0"},{"name":"v0.9.0"}]`))
	if err != nil {
		t.Fatalf("parseGHTags() error = %v", err)
	}
	if !tags["v1.0.0"] || !tags["v0.9.0"] {
		t.Fatalf("parseGHTags() = %v, want both tags", tags)
	}
	if tags["v0.8.0"] {
		t.Error("parseGHTags() reported a tag the remote does not carry")
	}
}

// TestReleaseNoteArgs checks the command each state of a release is written
// with: a release is created with the tag verified and its place among the
// releases stated, and one already there only has its body rewritten.
func TestReleaseNoteArgs(t *testing.T) {
	tests := []struct {
		name string
		note releaseNote
		want string
	}{
		{
			"the latest release, created",
			releaseNote{Tag: "v1.0.0", Title: "v1.0.0", Create: true, Latest: true},
			"gh release create v1.0.0 --verify-tag --title v1.0.0 --latest --notes-file -",
		},
		{
			"an older release, created",
			releaseNote{Tag: "v0.9.0", Title: "v0.9.0", Create: true},
			"gh release create v0.9.0 --verify-tag --title v0.9.0 --latest=false --notes-file -",
		},
		{
			"a release already there",
			releaseNote{Tag: "v0.9.0", Title: "v0.9.0"},
			"gh release edit v0.9.0 --notes-file -",
		},
		{
			"a nested module, released under its subdirectory",
			releaseNote{Tag: "worktree/v0.9.0", Title: "worktree/v0.9.0", Create: true},
			"gh release create worktree/v0.9.0 --verify-tag --title worktree/v0.9.0 --latest=false --notes-file -",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := strings.Join(test.note.args(), " "); got != test.want {
				t.Fatalf("releaseNote.args() = %q, want %q", got, test.want)
			}
		})
	}
}

// TestWriteReleaseNoteRunsInTheRepository checks the gh call is made in the
// repository the note belongs to, so no run of it depends on where worktree
// itself was started.
func TestWriteReleaseNoteRunsInTheRepository(t *testing.T) {
	out := stubGH(t, "pwd -P > \"$STUB_OUT\"\n")
	dir := t.TempDir()

	if _, err := writeReleaseNote(dir, releaseNote{Tag: "v1.0.0"}); err != nil {
		t.Fatalf("writeReleaseNote() error = %v", err)
	}

	// The temporary directory may be reached through a symlink, which pwd -P
	// resolves and the name of the directory does not.
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(written)); got != want {
		t.Fatalf("writeReleaseNote() ran gh in %q, want %q", got, want)
	}
}
