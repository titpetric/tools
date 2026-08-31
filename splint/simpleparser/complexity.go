package simpleparser

import (
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// complexity counts how much branching a body holds.
//
// Cyclomatic complexity is one plus the number of decision points, which are
// the branching keywords and the boolean operators between them. Counting them
// over tokens reproduces what gocyclo counts over a syntax tree, since gocyclo
// counts the same nodes.
//
// Cognitive complexity does not reduce to a count. gocognit weights a branch by
// how deeply it nests and treats a chain of boolean operators as one, and
// neither survives without a tree, so what is counted here is the branching
// with the nesting weight and no more. It is the one field of the model the two
// parsers are not expected to agree on.
func complexity(src *source, from, to int, text string) *model.Complexity {
	lines := strings.Count(text, "\n")
	if len(text) > 0 && !strings.HasSuffix(text, "\n") {
		lines++
	}

	cyclomatic, cognitive := 1, 0
	depth := 0

	for i := from; i <= to && i < src.len(); i++ {
		code := src.codeLine(i)
		trimmed := strings.TrimSpace(code)
		if trimmed == "" {
			continue
		}

		// The closing brace of a block is written before its contents are
		// counted, so a line that only closes lowers the nesting first.
		closes := strings.Count(code, "}")
		opens := strings.Count(code, "{")
		if strings.HasPrefix(trimmed, "}") {
			depth -= closes
			if depth < 0 {
				depth = 0
			}
			closes = 0
		}

		branches := branchCount(code)
		cyclomatic += branches
		cognitive += branches * (1 + depth)

		depth += opens - closes
		if depth < 0 {
			depth = 0
		}
	}

	return &model.Complexity{
		Cognitive:  cognitive,
		Cyclomatic: cyclomatic,
		Lines:      lines,
	}
}

// branchKeywords are the keywords that decide something, which is what a
// decision point is.
var branchKeywords = []string{"if", "for", "case", "range", "select"}

// branchCount is how many decisions one line makes.
func branchCount(code string) int {
	count := 0

	for _, keyword := range branchKeywords {
		count += keywordCount(code, keyword)
	}
	// A boolean operator between two conditions is another decision, since
	// either side can be what decided it.
	count += strings.Count(code, "&&") + strings.Count(code, "||")

	// A range clause sits inside a for, and the two are one decision.
	if keywordCount(code, "for") > 0 && keywordCount(code, "range") > 0 {
		count -= keywordCount(code, "range")
	}
	// "select" itself decides nothing; its cases do, and they are counted.
	count -= keywordCount(code, "select")

	return count
}

// keywordCount is how many times a word appears on a line as a word, rather
// than as part of a longer identifier.
func keywordCount(code, keyword string) int {
	count := 0

	for i := 0; i+len(keyword) <= len(code); i++ {
		if code[i:i+len(keyword)] != keyword {
			continue
		}
		if i > 0 && isIdentifierByte(code[i-1]) {
			continue
		}
		if end := i + len(keyword); end < len(code) && isIdentifierByte(code[end]) {
			continue
		}
		count++
		i += len(keyword) - 1
	}

	return count
}
