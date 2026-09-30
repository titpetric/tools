package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ghRelease is one release of a GitHub repository, as the API reports it.
type ghRelease struct {
	// TagName is the tag the release is named by, which is what a note is
	// matched to its release on.
	TagName string `json:"tag_name"`

	// Name is the title shown above the body, and Body is the note itself.
	Name string `json:"name"`
	Body string `json:"body"`

	// Draft reports a release nobody but the repository's own writers can see.
	// A draft is still a release to write into rather than one to create again.
	Draft bool `json:"draft"`
}

// ghTag is one tag of a GitHub repository, as the API reports it.
type ghTag struct {
	Name string `json:"name"`
}

// ghReleases returns the releases the remote of dir carries, keyed on the tag
// each one names.
//
// The whole list is read in one paginated call rather than one call per tag: a
// repository with forty tags is forty round trips otherwise, and the body is
// what a note is compared against, which "gh release list" does not report.
func ghReleases(dir string) (map[string]ghRelease, error) {
	out, err := ghJSON(dir, "api", "--paginate", "repos/{owner}/{repo}/releases?per_page=100")
	if err != nil {
		return nil, err
	}
	return parseGHReleases(out)
}

// parseGHReleases reads the release list the API answers with, keyed on the tag
// each release names.
func parseGHReleases(data []byte) (map[string]ghRelease, error) {
	var releases []ghRelease
	if err := json.Unmarshal(data, &releases); err != nil {
		return nil, fmt.Errorf("read the releases of the repository: %w", err)
	}

	byTag := make(map[string]ghRelease, len(releases))
	for _, release := range releases {
		byTag[release.TagName] = release
	}
	return byTag, nil
}

// ghTags returns the tags the remote of dir carries.
//
// A tag that was never pushed can hold no release: gh would create the tag at
// the head of the default branch, which for a note about an old release is a
// tag pointing at the wrong commit.
func ghTags(dir string) (map[string]bool, error) {
	out, err := ghJSON(dir, "api", "--paginate", "repos/{owner}/{repo}/tags?per_page=100")
	if err != nil {
		return nil, err
	}
	return parseGHTags(out)
}

// parseGHTags reads the tag list the API answers with, as the set of names it
// holds.
func parseGHTags(data []byte) (map[string]bool, error) {
	var tags []ghTag
	if err := json.Unmarshal(data, &tags); err != nil {
		return nil, fmt.Errorf("read the tags of the repository: %w", err)
	}

	pushed := make(map[string]bool, len(tags))
	for _, tag := range tags {
		pushed[tag.Name] = true
	}
	return pushed, nil
}

// ghJSON runs a gh command in dir and returns what it wrote to standard output.
// What it wrote to standard error becomes the error instead, since that is
// where gh explains an unauthenticated run or a repository it cannot resolve.
func ghJSON(dir string, args ...string) ([]byte, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, fmt.Errorf("gh is not installed")
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("gh", args...)
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		reason := strings.TrimSpace(stderr.String())
		if reason == "" {
			reason = err.Error()
		}
		return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), firstLine(reason))
	}
	return stdout.Bytes(), nil
}

// releaseNote is the note of one release as it is written to the remote.
type releaseNote struct {
	// Tag names the release, and Title is the heading shown above the body.
	Tag   string
	Title string

	// Body is the note, which is the markdown a verdict writes.
	Body string

	// Create tells a release that has to be made from one that is already
	// there and only needs its body rewritten.
	Create bool

	// Latest reports that the release carries the "Latest" badge, which is the
	// newest release of the repository and no other.
	Latest bool
}

// args returns the gh command that writes the note.
//
// The body is not among them: it goes to gh on standard input, since a note
// runs to thousands of characters and an argument list is no place for one.
//
// A release is created with --verify-tag, so gh refuses a tag the remote does
// not carry rather than creating one at the head of the default branch, which
// is what it does otherwise and is never what a note about an old release
// wants. An edit names no tag and can create none, so it does not need the
// flag, and passing it would refuse the draft of a tag that is not pushed yet.
//
// Whether the release is the latest one is stated outright on every release
// created, so writing a note for a release made a year ago does not take the
// badge from the release that holds it.
func (n releaseNote) args() []string {
	if !n.Create {
		return []string{"gh", "release", "edit", n.Tag, "--notes-file", "-"}
	}

	latest := "--latest=false"
	if n.Latest {
		latest = "--latest"
	}
	return []string{"gh", "release", "create", n.Tag, "--verify-tag", "--title", n.Title, latest, "--notes-file", "-"}
}

// writeReleaseNote runs the command that writes a note to the remote, handing
// it the body on standard input, and returns what gh printed.
func writeReleaseNote(dir string, note releaseNote) (string, error) {
	args := note.args()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(note.Body)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
