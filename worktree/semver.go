package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Version is a parsed semantic version tag. Prefix holds the optional "v"
// of the original tag, so an incremented version is printed the way the
// repository already tags its releases.
type Version struct {
	Prefix     string
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Build      string
}

// ParseVersion parses a tag such as "v1.2.3", "1.2.3-rc.1" or "v1.2.3+meta".
// It reports false for anything that isn't a strict major.minor.patch version.
func ParseVersion(tag string) (Version, bool) {
	s := strings.TrimSpace(tag)
	if s == "" {
		return Version{}, false
	}

	var v Version
	if s[0] == 'v' || s[0] == 'V' {
		v.Prefix, s = s[:1], s[1:]
	}
	if base, build, ok := strings.Cut(s, "+"); ok {
		if build == "" {
			return Version{}, false
		}
		s, v.Build = base, build
	}
	if base, pre, ok := strings.Cut(s, "-"); ok {
		if pre == "" {
			return Version{}, false
		}
		s, v.Prerelease = base, pre
	}

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, false
	}
	nums := make([]int, len(parts))
	for i, part := range parts {
		// Reject empty, signed, and zero-padded numbers.
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return Version{}, false
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return Version{}, false
		}
		nums[i] = n
	}
	v.Major, v.Minor, v.Patch = nums[0], nums[1], nums[2]
	return v, true
}

// ParseVersions parses every tag, silently dropping the ones that aren't
// semantic versions.
func ParseVersions(tags []string) []Version {
	versions := make([]Version, 0, len(tags))
	for _, tag := range tags {
		if v, ok := ParseVersion(tag); ok {
			versions = append(versions, v)
		}
	}
	return versions
}

// String renders the version the way it would be tagged.
func (v Version) String() string {
	s := fmt.Sprintf("%s%d.%d.%d", v.Prefix, v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		s += "-" + v.Prerelease
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// IsRelease reports whether the version is a release, that is, not a
// prerelease such as "v1.2.3-rc.1".
func (v Version) IsRelease() bool {
	return v.Prerelease == ""
}

// BumpPatch returns the next patch version, dropping any prerelease and
// build metadata.
func (v Version) BumpPatch() Version {
	return Version{Prefix: v.Prefix, Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}
}

// BumpMinor returns the next minor version, dropping any prerelease and
// build metadata.
func (v Version) BumpMinor() Version {
	return Version{Prefix: v.Prefix, Major: v.Major, Minor: v.Minor + 1}
}

// Compare orders two versions by semantic version precedence, returning -1,
// 0 or 1. Build metadata is ignored, a prerelease sorts before its release.
func Compare(a, b Version) int {
	for _, pair := range [][2]int{{a.Major, b.Major}, {a.Minor, b.Minor}, {a.Patch, b.Patch}} {
		if pair[0] != pair[1] {
			if pair[0] < pair[1] {
				return -1
			}
			return 1
		}
	}
	return comparePrerelease(a.Prerelease, b.Prerelease)
}

// comparePrerelease compares prerelease strings per the semver spec: an
// absent prerelease has the higher precedence, identifiers are compared
// field by field, and numeric fields sort before alphanumeric ones.
func comparePrerelease(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return 1
	}
	if b == "" {
		return -1
	}

	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if as[i] == bs[i] {
			continue
		}
		an, aErr := strconv.Atoi(as[i])
		bn, bErr := strconv.Atoi(bs[i])
		switch {
		case aErr == nil && bErr == nil:
			if an < bn {
				return -1
			}
			return 1
		case aErr == nil:
			return -1
		case bErr == nil:
			return 1
		case as[i] < bs[i]:
			return -1
		default:
			return 1
		}
	}
	switch {
	case len(as) < len(bs):
		return -1
	case len(as) > len(bs):
		return 1
	}
	return 0
}

// SortVersions sorts versions in ascending precedence order, in place.
func SortVersions(versions []Version) {
	sort.SliceStable(versions, func(i, j int) bool {
		if c := Compare(versions[i], versions[j]); c != 0 {
			return c < 0
		}
		return versions[i].String() < versions[j].String()
	})
}

// LatestRelease returns the highest release version among the tags,
// ignoring prereleases and tags that aren't semantic versions. It reports
// false when the tags hold no release.
func LatestRelease(tags []string) (Version, bool) {
	var releases []Version
	for _, v := range ParseVersions(tags) {
		if v.IsRelease() {
			releases = append(releases, v)
		}
	}
	if len(releases) == 0 {
		return Version{}, false
	}
	SortVersions(releases)
	return releases[len(releases)-1], true
}
