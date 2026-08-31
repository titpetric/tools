package grouping

import (
	"path"
	"strings"
)

// allowlist are the filenames that hold anything.
//
// A package collects its data types in one place often enough that naming that
// place after every type in it would be worse than naming it after none of
// them.
var allowlist = []string{
	"model*.go",
	"types*.go",
}

// checkPatterns reports whether any of the acceptable filenames covers the one
// the symbol is in. A pattern is a glob because a name ending in a star stands
// for the file and everything that starts like it, and the stem is compared as
// well so a pattern carrying no star still matches an exact name.
func checkPatterns(patterns []string, base, baseStem string) bool {
	for _, pattern := range patterns {
		if matched, err := path.Match(pattern, base); err == nil && matched {
			return true
		}
		if baseStem == strings.TrimSuffix(pattern, ".go") {
			return true
		}
	}
	return false
}

// matchFilenames is every filename a symbol of this name and receiver is
// allowed in.
//
// The list opens on the fullest name, receiver and name together, and walks
// back one word at a time, so Service.DiscoveryGet reaches
// service_discovery_get.go, service_discovery.go and service.go. Each of those
// also stands for its singular and for the noun under a doer suffix, which is
// how Assets reaches asset.go and Checker reaches check.go. Once the walk is
// down to two words the shorter names carry a star, because a file named for
// half a symbol is a file the rest of the symbol lives in too.
func matchFilenames(name, receiver, defaultFile string) []string {
	var result, partials []string

	// A constructor is named for what it returns, so New comes off before the
	// name is read: NewServer belongs in server.go.
	name = strings.TrimPrefix(name, "New")

	if name != "" {
		name = strings.ToUpper(name[:1]) + name[1:]
	}

	snakeName := toSnake(receiver + name)
	for {
		lastIndex := strings.LastIndex(snakeName, "_")
		if lastIndex == -1 {
			break
		}

		partials = append(partials, snakeName)
		snakeName = snakeName[:lastIndex]
	}
	partials = append(partials, snakeName)

	var suffix string
	for _, partial := range partials {
		result = append(result, matchFilename(partial+suffix))

		if singular := getSingular(partial); singular != partial {
			result = append(result, matchFilename(singular+suffix))
		}

		if base := getBaseNoun(partial); base != partial {
			result = append(result, matchFilename(base+suffix))
		}

		if strings.Count(partial, "_") == 1 {
			suffix = "*"
		}
	}

	// Errors are collected rather than filed one per type, and a package that
	// does that names the file for the collection.
	if name == "Error" || strings.Contains(receiver, "Err") {
		result = append(result, "errors.go")
	}

	// A compound receiver is also at home under its last word, so AnalyzerReport
	// reaches report.go as well as analyzer_report.go.
	if receiver != "" {
		if parts := splitCamelCase(receiver); len(parts) > 1 {
			result = append(result, matchFilename(parts[len(parts)-1]))
		}
	}

	result = append(result, defaultFile)

	return append(result, allowlist...)
}

// matchFilename is one name as a filename.
func matchFilename(name string) string {
	return toSnake(name) + ".go"
}
