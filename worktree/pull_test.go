package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestPullReposRendersGitDetails(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	repo := filepath.Join(root, "service")
	runGit(t, root, "init", "--bare", remote)
	runGit(t, root, "clone", remote, repo)
	runGit(t, repo, "config", "user.name", "Test User")
	runGit(t, repo, "config", "user.email", "test@example.com")
	writeTestFile(t, filepath.Join(repo, "README.md"), "test\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "initial")
	runGit(t, repo, "push", "-u", "origin", "HEAD")
	updater := filepath.Join(root, "updater")
	runGit(t, root, "clone", remote, updater)
	runGit(t, updater, "config", "user.name", "Test User")
	runGit(t, updater, "config", "user.email", "test@example.com")
	writeTestFile(t, filepath.Join(updater, "CHANGELOG.md"), "update\n")
	runGit(t, updater, "add", "CHANGELOG.md")
	runGit(t, updater, "commit", "-m", "update")
	runGit(t, updater, "push")

	var output bytes.Buffer
	pullRepos(&output, []string{repo}, false)
	got := output.String()
	for _, want := range []string{
		"| Path | Remote | Branch | Pull status |",
		"origin " + remote + " (fetch)",
		"Pulled 1 commit.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("pullRepos() output missing %q:\n%s", want, got)
		}
	}

	output.Reset()
	pullRepos(&output, []string{repo}, false)
	if got := output.String(); !strings.Contains(got, "Already up to date.") {
		t.Fatalf("second pullRepos() did not report an up-to-date repository:\n%s", got)
	}
}
