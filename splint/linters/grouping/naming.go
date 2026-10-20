package grouping

import (
	"strings"
	"unicode"
)

// toSnake converts an identifier into the snake case a filename spells it in,
// so HTTPClient reads as http_client, never h_t_t_p_client.
//
// This is the snake case of github.com/stoewer/go-strcase, written out here
// and not imported: the rule is four lines of ASCII case handling and a
// linter is not worth a dependency.
func toSnake(input string) string {
	input = strings.TrimSpace(input)
	buffer := make([]rune, 0, len(input)+3)

	var prev, curr rune
	for _, next := range input {
		switch {
		case isDelimiter(curr):
			// A run of delimiters collapses into the one that separates the
			// words either side of it.
			if !isDelimiter(prev) {
				buffer = append(buffer, '_')
			}
		case isUpper(curr):
			// A capital opens a word when a lower case letter comes before it,
			// and when it closes a run of capitals, the place where an
			// acronym ends and the next word begins.
			if isLower(prev) || (isUpper(prev) && isLower(next)) {
				buffer = append(buffer, '_')
			}
			buffer = append(buffer, toLower(curr))
		case curr != 0:
			buffer = append(buffer, toLower(curr))
		}
		prev, curr = curr, next
	}

	// The loop reads one rune behind, so the last one is still in hand.
	if len(input) > 0 {
		if isUpper(curr) && isLower(prev) && prev != 0 {
			buffer = append(buffer, '_')
		}
		buffer = append(buffer, toLower(curr))
	}

	return string(buffer)
}

// getSingular returns the singular of a word, which lets a symbol
// named for many of a thing sit in the file named for one of them: Assets.Get
// belongs in asset.go as much as in assets.go.
//
// It covers the plurals Go identifiers actually use and gets the
// rest close enough for a filename to match.
func getSingular(word string) string {
	if word == "" {
		return word
	}

	lower := strings.ToLower(word)

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

	if singular, known := irregulars[lower]; known {
		if word[0] >= 'A' && word[0] <= 'Z' {
			return strings.ToUpper(singular[:1]) + singular[1:]
		}
		return singular
	}

	switch {
	case strings.HasSuffix(lower, "ies") && len(word) > 3:
		// Cities reads as city, companies as company.
		return word[:len(word)-3] + "y"
	case strings.HasSuffix(lower, "sses"), strings.HasSuffix(lower, "xes"),
		strings.HasSuffix(lower, "ches"), strings.HasSuffix(lower, "shes"):
		// Classes reads as class, boxes as box, churches as church.
		return word[:len(word)-2]
	case strings.HasSuffix(lower, "es"):
		return word[:len(word)-2]
	case strings.HasSuffix(lower, "s") && !strings.HasSuffix(lower, "ss"):
		// Cars reads as car, and class does not read as clas.
		return word[:len(word)-1]
	}

	return word
}

// getBaseNoun strips the doer suffix off a name, so Fetcher reads as fetch and
// Runner as run. A type named for what it does belongs in the file named for
// the thing it does.
func getBaseNoun(word string) string {
	if !strings.HasSuffix(word, "er") || len(word) <= 2 {
		return word
	}

	base := word[:len(word)-2]

	// A suffix doubles the consonant before it, and the double comes back off
	// with the suffix: runner comes from run.
	if len(base) > 1 && base[len(base)-1] == base[len(base)-2] {
		base = base[:len(base)-1]
	}

	return base
}

// splitCamelCase splits an identifier on its capitals, so ServiceDiscovery
// comes back as Service and Discovery. A compound name may sit in the file
// named for either half of it.
func splitCamelCase(word string) []string {
	if word == "" {
		return nil
	}

	var (
		parts   []string
		current strings.Builder
	)

	for i, r := range word {
		if i > 0 && r >= 'A' && r <= 'Z' && current.Len() > 0 {
			parts = append(parts, current.String())
			current.Reset()
		}
		current.WriteRune(r)
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// isLower, isUpper and isDelimiter read the ASCII that spells a Go identifier,
// and toLower folds it.
func isLower(r rune) bool { return r >= 'a' && r <= 'z' }
func isUpper(r rune) bool { return r >= 'A' && r <= 'Z' }

func isDelimiter(r rune) bool {
	return r == '-' || r == '_' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func toLower(r rune) rune {
	if isUpper(r) {
		return r + 32
	}
	return r
}

// isExported reports a name a reader outside the package can reach, which in
// Go is a name opening on an upper case letter. The model keeps this to itself
// and a linter that reads names one at a time out of a block needs it, so this
// file spells it out again and does not reach for it.
func isExported(name string) bool {
	for _, r := range name {
		return unicode.IsUpper(r)
	}
	return false
}
