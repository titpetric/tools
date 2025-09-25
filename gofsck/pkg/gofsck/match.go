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
}

// match returns true if the symbol matches any expected filename patterns.
func match(symbol AnalyzerSymbol, baseName string) bool {
	// This reverses name and receiver for types.
	// It could be a better check based on symbol.Type.
	name, receiver := symbol.Symbol, symbol.Receiver
	if receiver == "" {
		receiver = name
		name = ""
	}

	expected := matchFilenames(name, receiver, symbol.Default)
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
	var result, partials []string

	// Constructors should start with New, followed with the type name.
	// We trim it away so we can group `NewServer` into server.go.
	if strings.HasPrefix(name, "New") {
		name = name[3:]
	}

	// make function name exported for naming checks
	if len(name) > 0 {
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

	var baseName, singularName, suffix string
	for _, name := range partials {
		result = append(result, matchFilename(name+suffix))

		singularName = getSingular(name)
		if singularName != name {
			result = append(result, matchFilename(singularName+suffix))
		}

		baseName = getBaseNoun(name)
		if baseName != name {
			result = append(result, matchFilename(baseName+suffix))
		}

		if strings.Count(name, "_") == 1 {
			suffix = "*"
		}
	}

	if name == "Error" || strings.Contains(receiver, "Err") {
		result = append(result, "errors.go")
	}

	result = append(result, defaultFile)

	return append(result, allowlist...)
}

// matchFilename applies normalization (snake_case) to match file naming convention.
func matchFilename(name string) string {
	return toSnake(name) + ".go"
}
