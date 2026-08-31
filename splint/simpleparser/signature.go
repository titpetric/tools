package simpleparser

import (
	"slices"
	"strings"
)

// splitTop splits a list on a separator at nesting depth zero, so a comma
// inside a parameter type does not separate two parameters: "func(a, b int)
// error" is one argument and not two.
//
// Parens, brackets and braces all nest, which covers a func type, a map key, a
// slice, a generic instantiation and an anonymous struct alike.
func splitTop(list string, sep byte) []string {
	var (
		parts []string
		depth int
		start int
	)

	for i := 0; i < len(list); i++ {
		switch list[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case sep:
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(list[start:i]))
				start = i + 1
			}
		}
	}

	if rest := strings.TrimSpace(list[start:]); rest != "" || len(parts) > 0 {
		parts = append(parts, rest)
	}
	return parts
}

// matchParen returns the index of the paren closing the one at open, and -1
// when the line does not close it.
func matchParen(line string, open int) int {
	depth := 0
	for i := open; i < len(line); i++ {
		switch line[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// paramGroups reads a parameter list into its groups, one entry per group as
// it is declared: "a, b int, c string" is two groups and not three parameters.
//
// The group is what the model counts, because the ast collector counts the
// fields of the parameter list and a field is a group. Each entry is the names
// it declares, joined by a comma, and the type they share.
func paramGroups(list string) []paramGroup {
	list = strings.TrimSpace(list)
	if list == "" {
		return nil
	}

	items := splitTop(list, ',')
	if len(items) == 0 {
		return nil
	}

	// Go requires the whole list named or none of it, so the last item decides
	// for all of them: "a, b int" is one group of two names, and "Trace, bool"
	// is two types with no names at all.
	if !namedList(items) {
		groups := make([]paramGroup, 0, len(items))
		for _, item := range items {
			if item != "" {
				groups = append(groups, paramGroup{Type: item})
			}
		}
		return groups
	}

	var (
		groups  []paramGroup
		pending []string
	)

	for _, item := range items {
		if item == "" {
			continue
		}

		name, typ := splitParam(item)
		if typ == "" {
			pending = append(pending, name)
			continue
		}

		groups = append(groups, paramGroup{Names: append(pending, name), Type: typ})
		pending = nil
	}

	return groups
}

// namedList reports whether a parameter or result list names what it declares,
// which the last item of it settles: a named list ends on a name and a type,
// and an unnamed one ends on a type alone.
func namedList(items []string) bool {
	last := strings.TrimSpace(items[len(items)-1])
	if last == "" {
		return false
	}

	space := indexTop(last, ' ')
	if space < 0 {
		return false
	}
	return isIdentifier(strings.TrimSpace(last[:space]))
}

// paramGroup is one field of a parameter or result list: the names it declares
// and the type they share.
type paramGroup struct {
	Names []string
	Type  string
}

// String renders the group the way a signature writes it.
func (g paramGroup) String() string {
	if len(g.Names) == 0 {
		return g.Type
	}
	return strings.Join(g.Names, ",") + " " + g.Type
}

// splitParam separates one parameter into its name and its type. A parameter
// with no name of its own reports an empty type, which is what tells a bare
// name in a group from a type standing alone.
func splitParam(param string) (name, typ string) {
	param = strings.TrimSpace(param)
	if param == "" {
		return "", ""
	}

	// A name is one identifier, and the type is whatever follows it. Anything
	// that is not a plain identifier followed by a space is a type on its own.
	space := indexTop(param, ' ')
	if space < 0 {
		if isIdentifier(param) {
			return param, ""
		}
		return "", param
	}

	head := param[:space]
	if !isIdentifier(head) {
		return "", param
	}
	return head, strings.TrimSpace(param[space+1:])
}

// indexTop is the index of the first separator at nesting depth zero.
func indexTop(list string, sep byte) int {
	depth := 0
	for i := 0; i < len(list); i++ {
		switch list[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case sep:
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// isIdentifier reports whether a word is a plain Go identifier, which is what
// a parameter name is and what a type expression is not.
func isIdentifier(word string) bool {
	if word == "" {
		return false
	}
	for i := 0; i < len(word); i++ {
		c := word[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_':
		case c >= '0' && c <= '9' && i > 0:
		case c >= 0x80:
			// A non ASCII byte is part of a unicode identifier.
		default:
			return false
		}
	}
	return !isKeyword(word)
}

// isKeyword reports whether a word is one a parameter cannot be named.
func isKeyword(word string) bool {
	switch word {
	case "func", "chan", "map", "struct", "interface", "any", "error", "string",
		"bool", "byte", "rune", "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float32", "float64", "complex64", "complex128":
		return true
	}
	return false
}

// resultGroups reads a result list into its groups.
//
// A single result needs no parens, a list of them does, and either may name
// its results.
func resultGroups(results string) []paramGroup {
	results = strings.TrimSpace(results)
	if results == "" {
		return nil
	}

	if strings.HasPrefix(results, "(") && matchParen(results, 0) == len(results)-1 {
		return paramGroups(results[1 : len(results)-1])
	}
	return []paramGroup{{Type: results}}
}

// groupTypes is the type of each group, deduplicated, keeping the first of
// each. It is what the model records as Arguments and Returns: the ast
// collector reads one type per field and drops a repeat.
func groupTypes(groups []paramGroup) []string {
	var types []string
	for _, group := range groups {
		typ := symbolType(group.Type)
		if typ == "" || slices.Contains(types, typ) {
			continue
		}
		types = append(types, typ)
	}
	return types
}

// signature renders the declaration the way the model records it: the name,
// the parameter groups in parens, and the results after them, bracketed when
// there is more than one.
func signature(name string, params, results []paramGroup) string {
	written := make([]string, 0, len(params))
	for _, group := range params {
		written = append(written, group.String())
	}

	// A result is written as its type, whatever it is named: the collector
	// prints the type of the field and never the names in front of it.
	var returns string
	switch len(results) {
	case 0:
	case 1:
		returns = results[0].Type
	default:
		rendered := make([]string, 0, len(results))
		for _, group := range results {
			rendered = append(rendered, group.Type)
		}
		returns = "(" + strings.Join(rendered, ", ") + ")"
	}

	if returns != "" {
		return name + " (" + strings.Join(written, ", ") + ") " + returns
	}
	return name + " (" + strings.Join(written, ", ") + ")"
}

// symbolType normalises a type the way the ast collector renders it. An
// anonymous interface is written as "any", whatever it declares: the collector
// reduces every interface type to that.
func symbolType(typ string) string {
	typ = strings.TrimSpace(typ)
	if strings.HasPrefix(typ, "interface{") || typ == "interface {}" || typ == "interface{}" {
		return "any"
	}

	// An array is written as a slice of its element: the collector prints "[]"
	// and what follows, so "[16]byte" and "[]byte" are the same to it.
	if strings.HasPrefix(typ, "[") {
		if close := strings.Index(typ, "]"); close > 1 && !strings.HasPrefix(typ, "[]") {
			if inner := strings.TrimSpace(typ[1:close]); !strings.Contains(inner, "]") {
				return "[]" + symbolType(typ[close+1:])
			}
		}
	}

	return typ
}
