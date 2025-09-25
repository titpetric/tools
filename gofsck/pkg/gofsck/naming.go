package gofsck

import (
	"strings"

	strcase "github.com/stoewer/go-strcase"
)

func toSnake(input string) string {
	return strcase.SnakeCase(input)
}

// getSingular returns the singular form of a word
// This is a simple implementation that handles common cases
func getSingular(word string) string {
	lower := strings.ToLower(word)

	// Common irregular plurals
	irregulars := map[string]string{
		"children": "child",
		"geese":    "goose",
		"men":      "man",
		"women":    "woman",
		"teeth":    "tooth",
		"feet":     "foot",
		"mice":     "mouse",
		"people":   "person",
	}

	if singular, ok := irregulars[lower]; ok {
		// Preserve original case for first letter
		if word[0] >= 'A' && word[0] <= 'Z' {
			return strings.Title(singular)
		}
		return singular
	}

	// Regular plural patterns
	if strings.HasSuffix(lower, "ies") && len(word) > 3 {
		// cities -> city, companies -> company
		return word[:len(word)-3] + "y"
	} else if strings.HasSuffix(lower, "es") {
		if strings.HasSuffix(lower, "sses") || strings.HasSuffix(lower, "xes") ||
			strings.HasSuffix(lower, "ches") || strings.HasSuffix(lower, "shes") {
			// classes -> class, boxes -> box, churches -> church, brushes -> brush
			return word[:len(word)-2]
		}
		// heroes -> hero (but not always correct)
		return word[:len(word)-2]
	} else if strings.HasSuffix(lower, "s") && !strings.HasSuffix(lower, "ss") {
		// cars -> car, but not class -> clas
		return word[:len(word)-1]
	}

	return word
}

// getBaseNoun extracts the base noun from "doer" patterns
// For example: Fetcher -> fetch, Checker -> check
func getBaseNoun(word string) string {
	if strings.HasSuffix(word, "er") && len(word) > 2 {
		base := word[:len(word)-2]
		// Handle doubling of consonants (e.g., runner -> run)
		if len(base) > 1 && base[len(base)-1] == base[len(base)-2] {
			base = base[:len(base)-1]
		}
		return base
	}
	return word
}
