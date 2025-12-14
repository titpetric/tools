package grouping

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

// matchWithOptions returns the match result, canonical locations, and total count of expected filenames.
func matchWithOptions(symbol AnalyzerSymbol, baseName string) (bool, []string, int) {
	name, receiver := symbol.Symbol, symbol.Receiver
	if receiver == "" {
		receiver = name
		name = ""
	}

	base := filepath.Base(baseName)
	baseStem := strings.TrimSuffix(base, ".go")

	// Generate canonical locations (K.V, K, V)
	var canonicalLocations []string
	if name != "" && receiver != "" {
		// K.V - receiver + name
		canonicalLocations = append(canonicalLocations, toSnake(receiver+name)+".go")
	}
	if receiver != "" {
		// K - receiver only
		canonicalLocations = append(canonicalLocations, toSnake(receiver)+".go")
	}
	if name != "" {
		// V - name only
		canonicalLocations = append(canonicalLocations, toSnake(name)+".go")
	}

	// Get all expected filenames to count them
	allExpected := matchFilenames(name, receiver, symbol.Default)

	// First try patterns with receiver + name
	if name != "" {
		expected := matchFilenames(name, receiver, symbol.Default)
		if checkPatterns(expected, base, baseStem) {
			return true, canonicalLocations, len(allExpected)
		}
	}

	// If no match found and we have a camelCase receiver, try each part with the name
	// e.g., ServiceDiscovery.Start -> [service_start.go, service.go, discovery_start.go, discovery.go]
	if name != "" && receiver != "" {
		receiverParts := splitCamelCase(receiver)
		for _, part := range receiverParts {
			partPatterns := matchFilenames(name, part, symbol.Default)
			if checkPatterns(partPatterns, base, baseStem) {
				return true, canonicalLocations, len(allExpected)
			}
		}
	}

	// If still no match, try each part of the function name separately
	// e.g., ServiceDiscovery.FooClient -> [foo_client.go, foo.go, client.go]
	if name != "" && receiver != "" {
		nameParts := splitCamelCase(name)
		for _, part := range nameParts {
			// Try just this part of the name with receiver
			partNamePatterns := matchFilenames(part, receiver, symbol.Default)
			if checkPatterns(partNamePatterns, base, baseStem) {
				return true, canonicalLocations, len(allExpected)
			}

			// Try each part of the name alone
			justPartPatterns := matchFilenames(part, "", symbol.Default)
			if checkPatterns(justPartPatterns, base, baseStem) {
				return true, canonicalLocations, len(allExpected)
			}
		}
	}

	// If still no match, try just the function name
	if name != "" && receiver != "" {
		justNamePatterns := matchFilenames(name, "", symbol.Default)
		if checkPatterns(justNamePatterns, base, baseStem) {
			return true, canonicalLocations, len(allExpected)
		}
	}

	return false, canonicalLocations, len(allExpected)
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

	base := filepath.Base(baseName)
	baseStem := strings.TrimSuffix(base, ".go")

	// First try patterns with receiver + name
	if name != "" {
		expected := matchFilenames(name, receiver, symbol.Default)
		if checkPatterns(expected, base, baseStem) {
			return true
		}
	}

	// If no match found and we have a camelCase receiver, try each part with the name
	// e.g., ServiceDiscovery.Start -> [service_start.go, service.go, discovery_start.go, discovery.go]
	if name != "" && receiver != "" {
		receiverParts := splitCamelCase(receiver)
		for _, part := range receiverParts {
			partPatterns := matchFilenames(name, part, symbol.Default)
			if checkPatterns(partPatterns, base, baseStem) {
				return true
			}
		}
	}

	// If still no match, try each part of the function name separately
	// e.g., ServiceDiscovery.FooClient -> [foo_client.go, foo.go, client.go]
	if name != "" && receiver != "" {
		nameParts := splitCamelCase(name)
		for _, part := range nameParts {
			// Try just this part of the name with receiver
			partNamePatterns := matchFilenames(part, receiver, symbol.Default)
			if checkPatterns(partNamePatterns, base, baseStem) {
				return true
			}

			// Try each part of the name alone
			justPartPatterns := matchFilenames(part, "", symbol.Default)
			if checkPatterns(justPartPatterns, base, baseStem) {
				return true
			}
		}
	}

	// If still no match, try just the function name
	if name != "" && receiver != "" {
		justNamePatterns := matchFilenames(name, "", symbol.Default)
		if checkPatterns(justNamePatterns, base, baseStem) {
			return true
		}
	}

	return false
}

// checkPatterns checks if any pattern matches the base filename
func checkPatterns(patterns []string, base, baseStem string) bool {
	for _, pattern := range patterns {
		// Try glob pattern matching
		matched, err := path.Match(pattern, base)
		if err == nil && matched {
			return true
		}

		// Also try exact snake_case matching of the filename stem
		// e.g., Vue.evalAttributes should match eval_attributes.go
		patternStem := strings.TrimSuffix(pattern, ".go")
		if baseStem == patternStem {
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
