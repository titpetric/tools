package main

import (
	"flag"
	"io"
	"os"
	"reflect"
	"testing"
)

func TestReleaseCommands(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		kind string
		want []string
	}{
		{
			name: "patch",
			tags: []string{"v1.2.3", "v1.2.0", "v1.3.0-rc.1"},
			kind: releasePatch,
			want: []string{"git tag v1.2.4", "git push --tags"},
		},
		{
			name: "minor",
			tags: []string{"v1.2.3", "v1.2.0"},
			kind: releaseMinor,
			want: []string{"git tag v1.3.0", "git push --tags"},
		},
		{
			name: "unprefixed tags",
			tags: []string{"1.2.3"},
			kind: releasePatch,
			want: []string{"git tag 1.2.4", "git push --tags"},
		},
		{
			name: "no releases",
			tags: []string{"nightly"},
			kind: releasePatch,
			want: []string{"# no release tags found, starting from v0.0.0", "git tag v0.0.1", "git push --tags"},
		},
		{
			name: "no releases minor",
			tags: nil,
			kind: releaseMinor,
			want: []string{"# no release tags found, starting from v0.0.0", "git tag v0.1.0", "git push --tags"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := releaseCommands(test.tags, test.kind, "")
			if err != nil {
				t.Fatalf("releaseCommands() error: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("releaseCommands() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestReleaseCommandsUnknownKind(t *testing.T) {
	if _, err := releaseCommands([]string{"v1.0.0"}, "major", ""); err == nil {
		t.Fatal("releaseCommands() accepted an unknown release kind")
	}
}

func TestParseOptionsRelease(t *testing.T) {
	originalArgs := os.Args
	originalFlags := flag.CommandLine
	t.Cleanup(func() {
		os.Args = originalArgs
		flag.CommandLine = originalFlags
	})

	for _, kind := range []string{releasePatch, releaseMinor} {
		os.Args = []string{"worktree", kind}
		flag.CommandLine = flag.NewFlagSet("worktree", flag.ContinueOnError)
		flag.CommandLine.SetOutput(io.Discard)

		opts := ParseOptions()
		if opts.Release != kind {
			t.Errorf("ParseOptions() Release = %q, want %q", opts.Release, kind)
		}
		if opts.FilterArg != "" || opts.FilterPath != "" {
			t.Errorf("ParseOptions() treated %q as a filter: %#v", kind, opts)
		}
	}
}

func TestGitTags(t *testing.T) {
	dir := t.TempDir()

	runGit(t, dir, "init", "--quiet")
	runGit(t, dir, "-c", "user.email=test@example.com", "-c", "user.name=test", "commit", "--allow-empty", "--quiet", "-m", "init")
	runGit(t, dir, "tag", "v0.1.0")
	runGit(t, dir, "tag", "v0.1.1")

	tags, err := gitTags(dir)
	if err != nil {
		t.Fatalf("gitTags() error: %v", err)
	}
	if want := []string{"v0.1.0", "v0.1.1"}; !reflect.DeepEqual(tags, want) {
		t.Fatalf("gitTags() = %v, want %v", tags, want)
	}

	got, err := releaseCommands(tags, releaseMinor, "")
	if err != nil {
		t.Fatalf("releaseCommands() error: %v", err)
	}
	if want := []string{"git tag v0.2.0", "git push --tags"}; !reflect.DeepEqual(got, want) {
		t.Errorf("releaseCommands() = %v, want %v", got, want)
	}
}
