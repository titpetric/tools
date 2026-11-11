package mdfmt

import (
	"os"
	"path/filepath"
	"strings"
)

// embeds reports whether a Go file in the directory embeds the markdown file
// at path.
//
// Markdown a program reads is data. `//go:embed *.md` beside a set of
// documents says a package parses them, and formatting a table in one of
// those breaks whatever reads it. The signal is exact and needs no setting:
// the directive names the files.
//
// Each directory answers once and keeps answering the same. A walk asks
// about every markdown file it meets. A directory holding forty of them
// would read its Go forty times.
type embedCache map[string][]string

// embedded reports whether a directive in dir claims the file at path.
func (c embedCache) embedded(dir, path string) bool {
	patterns, known := c[dir]
	if !known {
		patterns = embedPatterns(dir)
		c[dir] = patterns
	}

	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)

	for _, pattern := range patterns {
		if claims(pattern, rel) {
			return true
		}
	}

	return false
}

// claims reports whether one embed pattern names the file at rel.
//
// A pattern naming a directory takes everything under it. That is what
// `//go:embed docs` means. The `all:` prefix extends it to the files a
// plain directive skips.
func claims(pattern, rel string) bool {
	pattern = strings.TrimPrefix(pattern, "all:")

	if matched, err := filepath.Match(pattern, rel); err == nil && matched {
		return true
	}

	return strings.HasPrefix(rel, strings.TrimSuffix(pattern, "/")+"/")
}

// embedPatterns are the markdown patterns the Go files of one directory
// embed.
//
// It scans the text and parses nothing. A directive is a comment and the
// parser would answer the same line. A file that fails to parse still states
// what it embeds.
func embedPatterns(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var found []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}

		body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		for _, line := range strings.Split(string(body), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "//go:embed ") {
				continue
			}
			for _, pattern := range strings.Fields(strings.TrimPrefix(line, "//go:embed ")) {
				if markdownPattern(pattern) {
					found = append(found, pattern)
				}
			}
		}
	}

	return found
}

// markdownPattern reports whether an embed pattern could name a markdown
// file. A pattern ending in another extension names something else. A
// pattern naming a directory could hold anything.
func markdownPattern(pattern string) bool {
	trimmed := strings.TrimSuffix(strings.TrimPrefix(pattern, "all:"), "/")

	ext := filepath.Ext(trimmed)
	return ext == "" || ext == ".md" || ext == ".*"
}
