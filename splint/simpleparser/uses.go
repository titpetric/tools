package simpleparser

import (
	"sort"
)

// uses are the identifiers a file writes before a dot, sorted and
// deduplicated.
//
// It is read off the stripped view, so a dot inside a string or a comment is
// not a selector. Nothing else is filtered: a local variable, a parameter and
// a receiver all land in here alongside the package names, because the set
// exists to answer whether removing an import would break the file, and the
// only safe error is to keep one too many.
func uses(src *source) []string {
	seen := map[string]bool{}

	for i := 0; i < src.len(); i++ {
		eachSelector(src.codeLine(i), func(pkg, _ string) {
			seen[pkg] = true
		})
	}

	if len(seen) == 0 {
		return nil
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}
