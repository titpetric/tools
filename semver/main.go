package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"maps"
	"os"
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

func main() {
	var result TagList

	// Read from stdin (git ls-remote --tags origin)
	scanner := bufio.NewScanner(os.Stdin)

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
	majors = majors[0:2]

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
