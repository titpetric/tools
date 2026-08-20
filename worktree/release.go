package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// Release kinds accepted as a worktree subcommand.
const (
	releasePatch = "patch"
	releaseMinor = "minor"
)

// gitTags lists every tag in the git repository containing dir.
func gitTags(dir string) ([]string, error) {
	cmd := exec.Command("git", "tag", "--list")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var tags []string
	for _, line := range strings.Split(string(out), "\n") {
		if tag := strings.TrimSpace(line); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

// releaseCommands returns the git commands that tag and push the next patch
// or minor release for the given tags. The output is meant to be piped into
// "sh -x". When no release tag exists yet, the version starts at v0.0.0 and a
// shell comment records that.
func releaseCommands(tags []string, kind string) ([]string, error) {
	latest, found := LatestRelease(tags)
	if !found {
		latest = Version{Prefix: "v"}
	}

	var next Version
	switch kind {
	case releasePatch:
		next = latest.BumpPatch()
	case releaseMinor:
		next = latest.BumpMinor()
	default:
		return nil, fmt.Errorf("unknown release kind %q", kind)
	}

	var lines []string
	if !found {
		lines = append(lines, fmt.Sprintf("# no release tags found, starting from %s", latest))
	}
	return append(lines,
		fmt.Sprintf("git tag %s", next),
		"git push --tags",
	), nil
}
