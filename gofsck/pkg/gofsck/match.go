package gofsck

import (
	"path"
	"path/filepath"
	"strings"
)

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

	snakeName := toSnake(name)
	for {
		lastIndex := strings.LastIndex(snakeName, "_")
		if lastIndex == -1 {
			break
		}

		partials = append(partials, snakeName)
		snakeName = snakeName[:lastIndex]
	}
	partials = append(partials, snakeName)

	if receiver != "" {
		result := []string{}
		for _, partial := range partials {
			result = append(result, matchFilename(receiver+"_"+partial))
		}
		result = append(result, matchFilename(receiver+"*"), matchFilename(name), defaultFile)
		return result
	}

	result := []string{}
	suffix := ""
	for _, name := range partials {
		if strings.HasPrefix(name, "new_") {
			result = append(result, matchFilename(name[4:]+suffix))
		} else {
			result = append(result, matchFilename(name+suffix))
		}
		suffix = "*"
	}
	return append(result, defaultFile)
}

// matchFilename applies normalization (snake_case) to match file naming convention.
func matchFilename(name string) string {
	return toSnake(name) + ".go"
}
