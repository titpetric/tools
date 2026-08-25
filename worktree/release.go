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

// nextRelease returns the version the given tags reach after a patch or minor
// release, and whether the repository had a release tag to begin with. With
// none, the version starts at v0.0.0, so a patch proposes v0.0.1.
func nextRelease(tags []string, kind string) (next Version, found bool, err error) {
	latest, found := LatestRelease(tags)
	if !found {
		latest = Version{Prefix: "v"}
	}

	switch kind {
	case releasePatch:
		return latest.BumpPatch(), found, nil
	case releaseMinor:
		return latest.BumpMinor(), found, nil
	}
	return Version{}, found, fmt.Errorf("unknown release kind %q", kind)
}

// releaseSteps returns the git commands that tag and push the next patch or
// minor release, as the arguments to run them with. Printing and running a
// release share this, so the commands "worktree patch" prints are the ones
// "worktree resolve --apply" runs.
//
// The prefix is the one the tags of the module carry, which is "<subdir>/" for
// a module nested in a larger repository and empty for one at its root.
func releaseSteps(tags []string, kind, prefix string) ([][]string, error) {
	next, _, err := nextRelease(tags, kind)
	if err != nil {
		return nil, err
	}
	return [][]string{
		{"git", "tag", prefix + next.String()},
		{"git", "push", "--tags"},
	}, nil
}

// moduleTags returns the release tags of the module in dir, without the prefix
// they carry, and that prefix.
//
// A go module nested in a larger repository is released as "<subdir>/vX.Y.Z",
// which is how the go tool tells the releases of one module in a repository
// from another's. Stripping the prefix leaves plain versions to compare, and
// it is put back when a tag is created. A module at the root of its repository,
// and anything that is not a go module, is tagged "vX.Y.Z" and has no prefix.
func moduleTags(dir string) (tags []string, prefix string, err error) {
	all, err := gitTags(dir)
	if err != nil {
		return nil, "", err
	}

	_, rel, err := repoPaths(dir)
	if err != nil || rel == "." || !isGoModule(dir) {
		return all, "", nil
	}

	prefix = rel + "/"
	for _, tag := range all {
		if strings.HasPrefix(tag, prefix) {
			tags = append(tags, strings.TrimPrefix(tag, prefix))
		}
	}
	return tags, prefix, nil
}

// releaseCommands returns the git commands that tag and push the next patch
// or minor release for the given tags. The output is meant to be piped into
// "sh -x". When no release tag exists yet, the version starts at v0.0.0 and a
// shell comment records that.
func releaseCommands(tags []string, kind, prefix string) ([]string, error) {
	steps, err := releaseSteps(tags, kind, prefix)
	if err != nil {
		return nil, err
	}

	var lines []string
	if _, found := LatestRelease(tags); !found {
		lines = append(lines, fmt.Sprintf("# no release tags found, starting from %s", Version{Prefix: "v"}))
	}
	for _, step := range steps {
		lines = append(lines, strings.Join(step, " "))
	}
	return lines, nil
}
