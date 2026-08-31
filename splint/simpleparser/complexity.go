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

// branchCount is how many decisions one line makes.
//
// The line is walked once and every word on it is looked up, rather than the
// line being walked once per keyword: a decision is a word, and finding the
// words costs the same whether one is being looked for or six.
func branchCount(code string) int {
	var (
		count  int
		fors   int
		ranges int
	)

	for i := 0; i < len(code); i++ {
		c := code[i]

		// A boolean operator between two conditions is another decision,
		// since either side can be what decided it.
		if (c == '&' || c == '|') && i+1 < len(code) && code[i+1] == c {
			count++
			i++
			continue
		}

		if !isIdentifierByte(c) {
			continue
		}
		start := i
		for i < len(code) && isIdentifierByte(code[i]) {
			i++
		}
		if start > 0 && (isIdentifierByte(code[start-1]) || code[start-1] == '.') {
			continue
		}

		switch code[start:i] {
		case "if", "case":
			count++
		case "for":
			count++
			fors++
		case "range":
			count++
			ranges++
		}
		i--
	}

	// A range clause sits inside a for, and the two are one decision.
	if fors > 0 {
		count -= min(fors, ranges)
	}

	return count
}
