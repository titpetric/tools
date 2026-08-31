package simpleparser

import (
	"regexp"
	"strings"
)

// buildTagLine matches the old style build constraint, which is the one the
// ast parser looks for.
var buildTagLine = regexp.MustCompile(`(?m)^\s*//\s*\+build\s+(.*)$`)

// BuildTags returns the build constraints a file carries.
//
// A constraint that is a single negation is no constraint on the default
// build: "// +build !jq" means the file is read unless jq is asked for, and it
// is not, so the file is read.
func BuildTags(src []byte) []string {
	var tags []string

	for _, match := range buildTagLine.FindAllStringSubmatch(string(src), -1) {
		tags = append(tags, strings.TrimSpace(match[1]))
	}
	if len(tags) == 1 && strings.HasPrefix(tags[0], "!") {
		return nil
	}

	return tags
}
