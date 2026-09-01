package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
)

type Tag struct {
	Commit string
	Name   string
	Ref    string

	Version *semver.Version

	Major, Minor, Patch uint64
}

type TagList []*Tag

func (l TagList) Filter(cb func(*Tag) bool) TagList {
	var result []*Tag
	for _, v := range l {
		if cb(v) {
			result = append(result, v)
		}
	}
	return result
}

// helpSpec is the page the command prints.
func helpSpec(fs *flag.FlagSet) spec {
	return spec{
		Name:    "semver",
		Tagline: "the latest patch of every minor, over the last two majors",
		Usage: []string{
			"semver",
			"git ls-remote --tags <url> | semver",
		},
		Description: `Semver reads git tags and writes the ones worth scanning as JSON.

The tags come from stdin when stdin is a pipe, in the two column form that
"git ls-remote --tags" writes: a commit and a "refs/tags/" ref. With nothing
piped in it runs "git tag -l" in the current repository instead, and the
commit of every tag is then written as "local".

A tag is read as a version with the leading "v" taken off. Anything that is
not strict semver is skipped, and so is every prerelease and every "^{}" ref
of an annotated tag. What is left is grouped by major, the two highest majors
are kept, and within each of them the highest patch of each minor is kept.

The JSON is an array of objects, ordered by major and then minor, both
descending. Each one holds Commit, Name, Ref, Version, Major, Minor and
Patch. Name and Version are the version as it parsed, without the "v", and
Ref is the tag as it is written in the repository.`,
		Flags: fs,
		Examples: []example{
			{"semver", "the tags of the repository here, as JSON"},
			{"semver | jq -r '.[].Ref'", "the same tags, as a list of names"},
			{"git ls-remote --tags https://github.com/golang/go | semver", "the tags of a repository that is not checked out"},
			{"cat git-tags.txt | semver", "read ls-remote output that was saved to a file"},
		},
		Notes: `The tool takes no arguments and no flags: what it reads is decided by
whether stdin is a pipe.

Exits 1 when git fails, when the input cannot be read, or when the JSON
cannot be written.`,
	}
}

func main() {
	// A flag set with nothing in it still answers --help and -h, which is all
	// this tool has to read off a command line.
	fs := flag.NewFlagSet("semver", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			if err := writeHelp(os.Stdout, helpSpec(fs)); err != nil {
				fmt.Fprintln(os.Stderr, "Error writing help:", err)
				os.Exit(1)
			}
			return
		}
		fmt.Fprintln(os.Stderr, "Error parsing arguments:", err)
		os.Exit(1)
	}

	var result TagList

	var reader io.Reader = os.Stdin

	// Detect if stdin has piped data. If stdin is a pipe (FIFO), read from it.
	// Otherwise (terminal, /dev/null, etc.), fall back to git ls-remote.
	stdinIsPipe := false
	if info, err := os.Stdin.Stat(); err == nil {
		stdinIsPipe = (info.Mode() & os.ModeNamedPipe) != 0
	}

	if !stdinIsPipe {
		cmd := exec.Command("git", "tag", "-l")
		cmd.Stderr = os.Stderr
		out, err := cmd.Output()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error running git tag:", err)
			os.Exit(1)
		}
		// Convert local tag names to the refs/tags/ format expected by the parser.
		// Use "local" as a placeholder commit hash since we don't need it.
		var lines []string
		for _, tag := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				lines = append(lines, "local\trefs/tags/"+tag)
			}
		}
		reader = strings.NewReader(strings.Join(lines, "\n"))
	}

	scanner := bufio.NewScanner(reader)

	// Process each line of input
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) < 2 {
			continue
		}
		commit, ref := parts[0], parts[1]

		// Ensure the reference is a tag
		if !strings.HasPrefix(ref, "refs/tags/") {
			continue
		}
		// Skip annotated git tags
		if strings.Contains(ref, "^") {
			continue
		}

		ref = ref[10:]

		tag := strings.TrimPrefix(ref, "v")

		// Parse tag as a valid semver version
		v, err := semver.StrictNewVersion(tag)
		if err != nil {
			continue // Skip invalid semver tags
		}

		if v.Prerelease() != "" {
			continue
		}

		// Store valid semver tags and their commit hashes
		result = append(result, &Tag{
			Commit:  commit,
			Name:    v.Original(),
			Version: v,
			Major:   v.Major(),
			Minor:   v.Minor(),
			Patch:   v.Patch(),
			Ref:     ref,
		})
	}

	majorVersions := make(map[uint64]bool)
	for _, v := range result {
		majorVersions[v.Major] = true
	}

	majors := slices.Sorted(maps.Keys(majorVersions))

	slices.Reverse(majors)

	// keep last 2 major releases for scans
	if len(majors) > 2 {
		majors = majors[0:2]
	}

	var keep []*Tag

	for _, major := range majors {
		majorTags := result.Filter(func(t *Tag) bool {
			return t.Major == major
		})

		minorVersions := make(map[uint64]bool)
		for _, v := range majorTags {
			minorVersions[v.Minor] = true
		}

		minors := slices.Sorted(maps.Keys(minorVersions))

		slices.Reverse(minors)

		for _, minor := range minors {
			tags := majorTags.Filter(func(t *Tag) bool {
				return t.Minor == minor
			})

			// there has to be at least one
			var latestTag *Tag = tags[0]
			for _, tag := range tags {
				if latestTag.Version.Compare(semver.MustParse(tag.Name)) == -1 {
					latestTag = tag
				}
			}
			keep = append(keep, latestTag)
		}

	}

	result = keep

	// Handle any error while reading input
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Error reading input:", err)
		os.Exit(1)
	}

	// Output the result as JSON
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, "Error encoding JSON:", err)
		os.Exit(1)
	}
}
