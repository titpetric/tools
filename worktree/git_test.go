package main

import (
	"path/filepath"
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

func TestBrowsableRemote(t *testing.T) {
	tests := map[string]string{
		"git@github.com:titpetric/platform.git":       "https://github.com/titpetric/platform",
		"git@github.com:titpetric/platform":           "https://github.com/titpetric/platform",
		"ssh://git@github.com/titpetric/platform.git": "https://github.com/titpetric/platform",
		"https://github.com/titpetric/platform.git":   "https://github.com/titpetric/platform",
		"https://github.com/titpetric/platform":       "https://github.com/titpetric/platform",
		"http://git.example.com/team/project.git":     "http://git.example.com/team/project",
		"git@git.example.com:2222/team/project.git":   "https://git.example.com/2222/team/project",
		"  git@github.com:titpetric/platform.git\n":   "https://github.com/titpetric/platform",
		// Nothing a browser opens, so nothing to link to.
		"":                        "",
		"/srv/git/project.git":    "",
		"git://github.com/x/y":    "",
		"file:///srv/git/project": "",
	}

	for remote, want := range tests {
		if got := browsableRemote(remote); got != want {
			t.Errorf("browsableRemote(%q) = %q, want %q", remote, got, want)
		}
	}
}

func TestCommitLogCoversTheModuleOnly(t *testing.T) {
	root := testRepo(t, "alpha", "beta")
	runGit(t, root, "tag", "alpha/v0.1.0")

	writeTestFile(t, filepath.Join(root, "alpha", "alpha.go"), "package alpha\n\n// changed\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: change")
	writeTestFile(t, filepath.Join(root, "beta", "beta.go"), "package beta\n\n// changed\n")
	runGit(t, root, "commit", "--quiet", "-am", "beta: change")
	runGit(t, root, "tag", "alpha/v0.2.0")

	alpha := filepath.Join(root, "alpha")

	got := commitLogSinceTag(alpha, "alpha/v0.1.0")
	if len(got) != 1 || got[0].Subject != "alpha: change" {
		t.Fatalf("commitLogSinceTag() = %#v, want the one commit touching alpha", got)
	}
	if got[0].Hash == "" {
		t.Error("commitLogSinceTag() returned a commit without a hash")
	}

	// A range between two tags is what a released module reports on.
	if got := commitLogBetween(alpha, "alpha/v0.1.0", "alpha/v0.2.0"); len(got) != 1 {
		t.Errorf("commitLogBetween() = %d commits, want 1", len(got))
	}

	// Without a tag the whole history of the module is what a first release
	// would cover.
	if got := commitLogSinceTag(alpha, ""); len(got) != 2 {
		t.Errorf("commitLogSinceTag() without a tag = %d commits, want 2", len(got))
	}
}

func TestCommitLogMarksOnlyRemoteCommitsPublished(t *testing.T) {
	root := testRepo(t, "alpha")
	runGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	writeTestFile(t, filepath.Join(root, "alpha", "alpha.go"), "package alpha\n\n// local\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: local change")

	got := commitLogSinceTag(filepath.Join(root, "alpha"), "")
	if len(got) != 2 {
		t.Fatalf("commitLogSinceTag() = %#v, want two commits", got)
	}
	if got[0].Published {
		t.Errorf("local commit is marked published: %#v", got[0])
	}
	if !got[1].Published {
		t.Errorf("remote commit is marked unpublished: %#v", got[1])
	}
}
