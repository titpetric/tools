package gofsck

import (
	"path"
	"path/filepath"
	"strings"
)

// allowlist is a list of files to allow.
var allowlist = []string{
	"model*.go",
	"types*.go",
	"interface*.go",
	"const*.go",
	"func*.go",
}

// match returns true if the symbol matches any expected filename patterns.
func match(symbol AnalyzerSymbol, baseName string) bool {
	expected := matchFilenames(symbol.Symbol, symbol.Receiver, symbol.Default)
	base := filepath.Base(baseName)

	for _, name := range expected {
		matched, err := path.Match(name, base)
		if err == nil && matched {
			return true
		}
	}

	return false
}

// matchFilenames generates possible filename stems that could contain the given symbol.
func matchFilenames(name, receiver, defaultFile string) []string {
	partials := []string{}

	// Constructors should start with New, followed with the type name.
	// We trim it away so we can group `NewServer` into server.go.
	if len(name) > 3 && strings.EqualFold(name[:3], "New") {
		name = name[3:]
	}

	// make function name exported for naming checks
	name = strings.ToUpper(name[:1]) + name[1:]

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

	result := []string{}
	suffix := ""
	for _, name := range partials {
		result = append(result, matchFilename(name+suffix))

		suffix = "*"

		// Assets{} can be in asset.go.
		if strings.HasSuffix(name, "s") {
			result = append(result, matchFilename(name[:len(name)-1]+suffix))
		}

		// Checker{} can be in checker, checks, check.go.
		if strings.HasSuffix(name, "er") {
			result = append(result, matchFilename(name[:len(name)-2]+suffix))
		}
	}

	result = append(result, defaultFile)
	return append(result, allowlist...)
}

// matchFilename applies normalization (snake_case) to match file naming convention.
func matchFilename(name string) string {
	return toSnake(name) + ".go"
}
