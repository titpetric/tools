package simpleparser

import "strings"

// exits counts the returns and program-ending calls of a body outside its
// function literals, plus the closing brace when the last statement is neither.
// The body runs from line from to its closing brace on line to.
func exits(src *source, from, to int) int {
	if from > to {
		code := src.codeLine(to)
		if open := strings.Index(code, "{"); open >= 0 {
			code = code[open+1:]
		}
		if count, _, _ := scanExits(code, 0, nil); count > 0 {
			return count
		}
		return 1
	}

	var (
		count    int
		depth    int
		closures []closure
		terminal bool
		carried  bool
	)

	for i := from; i < to && i < src.len(); i++ {
		code := src.codeLine(i)
		trimmed := strings.TrimSpace(code)
		if trimmed == "" {
			continue
		}

		// A statement of the body opens at depth zero, and a line that closes
		// a block or a call, or follows an operator, continues the one above.
		if depth == 0 && len(closures) == 0 && !carried && !strings.HasPrefix(trimmed, "}") && !strings.HasPrefix(trimmed, ")") {
			terminal = endsFunction(trimmed)
		}
		raw, _ := src.split(i)
		carried = continues(strings.TrimSpace(raw))

		var found int
		found, depth, closures = scanExits(code, depth, closures)
		count += found
	}

	if !terminal {
		count++
	}
	return count
}

// closure is a function literal the scan is inside: the depth its func keyword
// sits at, and whether its body has opened.
type closure struct {
	depth  int
	opened bool
}

// scanExits counts the exits on one line outside any function literal, and
// carries the nesting of braces and parens to the next line.
func scanExits(code string, depth int, closures []closure) (int, int, []closure) {
	count := 0

	for i := 0; i < len(code); i++ {
		c := code[i]
		switch c {
		case '{', '(', '[':
			if n := len(closures); c == '{' && n > 0 && !closures[n-1].opened && depth == closures[n-1].depth {
				closures[n-1].opened = true
			}
			depth++
			continue
		case '}', ')', ']':
			depth--
			for n := len(closures); n > 0 && (depth < closures[n-1].depth || depth == closures[n-1].depth && c == '}' && closures[n-1].opened); n = len(closures) {
				closures = closures[:n-1]
			}
			continue
		}

		if !isIdentifierByte(c) {
			continue
		}
		start := i
		for i < len(code) && isIdentifierByte(code[i]) {
			i++
		}
		word := code[start:i]
		i--
		if start > 0 && code[start-1] == '.' {
			continue
		}
		if word == "func" {
			closures = append(closures, closure{depth: depth})
			continue
		}
		if len(closures) == 0 && exitWord(word, code[start+len(word):]) {
			count++
		}
	}

	// A func keyword whose body did not open on its line is a func type, and
	// no literal: gofmt keeps a literal's brace on the line of its keyword.
	for n := len(closures); n > 0 && !closures[n-1].opened; n = len(closures) {
		closures = closures[:n-1]
	}

	return count, depth, closures
}

// exitWord reports whether a word, with the code after it, leaves the
// function: return, panic, os.Exit and log.Fatal*.
func exitWord(word, rest string) bool {
	switch word {
	case "return":
		return true
	case "panic":
		return strings.HasPrefix(rest, "(")
	case "os":
		return strings.HasPrefix(rest, ".Exit(")
	case "log":
		return strings.HasPrefix(rest, ".Fatal")
	}
	return false
}

// continues reports whether a line ends on an operator, which carries its
// statement to the next line.
func continues(line string) bool {
	if line == "" || strings.HasSuffix(line, "++") || strings.HasSuffix(line, "--") {
		return false
	}
	return strings.ContainsAny(line[len(line)-1:], "|&.,+-*/%=<>^!:")
}

// endsFunction reports whether a statement leaves the function.
func endsFunction(stmt string) bool {
	return strings.HasPrefix(stmt, "return") && (len(stmt) == 6 || !isIdentifierByte(stmt[6])) ||
		strings.HasPrefix(stmt, "panic(") ||
		strings.HasPrefix(stmt, "os.Exit(") ||
		strings.HasPrefix(stmt, "log.Fatal")
}
