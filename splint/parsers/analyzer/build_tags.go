package analyzer

import (
	"regexp"
	"strings"
)

func BuildTags(src []byte) []string {
	// Regular expression to match build tags in comments.
	re := regexp.MustCompile(`(?m)^\s*//\s*\+build\s+(.*)$`)

	var buildTags []string

	// Find all occurrences of +build lines in comments.
	matches := re.FindAllStringSubmatch(string(src), -1)
	for _, match := range matches {
		buildTag := strings.TrimSpace(match[1])
		buildTags = append(buildTags, buildTag)
	}

	// If a build tag is `!jq` (starts with !), consider it
	// as if it's not present.
	if len(buildTags) == 1 && strings.HasPrefix(buildTags[0], "!") {
		return nil
	}

	return buildTags
}
