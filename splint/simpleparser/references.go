package simpleparser

import (
	"slices"
	"strings"
	"unicode"

	"github.com/titpetric/tools/splint/model"
)

// references reads the package symbols a body reaches, which is every
// "pkg.Symbol" where pkg is a name the file imports and Symbol is exported.
//
// The ast collector asks the same question a weaker way: it takes every
// selector whose left side is an identifier the parser did not resolve to a
// local. Reading the import table answers it outright, since a name that is
// not imported is not a package however it is written.
//
// The shadowed names are the parameters and the receiver, which are locals
// whatever else they are called: "func Run(config config.Config)" reaches its
// argument in the body and not the package it is named after.
func (f *file) references(src *source, from, column, to int, shadowed []string) model.StringSet {
	var refs model.StringSet

	for i := from; i <= to && i < src.len(); i++ {
		// The body of a one line func starts partway along the line it is
		// declared on, and what precedes it is the signature: a type named
		// there is not something the body reached.
		code := src.codeLine(i)
		if i == from && column > 0 && column < len(code) {
			code = strings.Repeat(" ", column) + code[column:]
		}

		for _, ref := range selectors(code) {
			if ref.pkg == "internal" || !f.aliases[ref.pkg] || !exported(ref.symbol) {
				continue
			}
			if slices.Contains(shadowed, ref.pkg) {
				continue
			}
			if refs == nil {
				refs = model.NewStringSet()
			}
			if !slices.Contains(refs[ref.pkg], ref.symbol) {
				refs.Add(ref.pkg, ref.symbol)
			}
		}
	}

	return refs
}

// selector is one "pkg.Symbol" as it was written.
type selector struct {
	pkg    string
	symbol string
}

// selectors returns every "left.Right" on a line, where both sides are plain
// identifiers, in the order they were written.
//
// A line reaching one package twice reports both: "http.Error(w, msg,
// http.StatusNotFound)" names two symbols of one package, and a set keyed on
// the package would keep one of them.
//
// A chained selector reports only its first pair: "a.B.C" reaches package a
// through B, and B.C is a field of whatever B returned rather than a second
// package.
func selectors(line string) []selector {
	var found []selector

	for i := 0; i < len(line); i++ {
		if line[i] != '.' {
			continue
		}

		left, start := identifierBefore(line, i)
		right, end := identifierAfter(line, i+1)
		if left == "" || right == "" {
			i = end
			continue
		}

		// A selector reached through another selector is not a package. So is
		// a number: "1.5" has an identifier on neither side, and a call
		// result, "f().X", has no identifier on the left.
		if start > 0 && (line[start-1] == '.' || isIdentifierByte(line[start-1])) {
			i = end
			continue
		}

		found = append(found, selector{pkg: left, symbol: right})
		i = end
	}

	return found
}

// identifierBefore reads the identifier ending at index, and where it starts.
func identifierBefore(line string, index int) (string, int) {
	end := index
	start := end
	for start > 0 && isIdentifierByte(line[start-1]) {
		start--
	}
	if start == end {
		return "", index
	}
	word := line[start:end]
	if word[0] >= '0' && word[0] <= '9' {
		return "", index
	}
	return word, start
}

// identifierAfter reads the identifier starting at index, and where it ends.
func identifierAfter(line string, index int) (string, int) {
	end := index
	for end < len(line) && isIdentifierByte(line[end]) {
		end++
	}
	if end == index {
		return "", index
	}
	word := line[index:end]
	if word[0] >= '0' && word[0] <= '9' {
		return "", end
	}
	return word, end
}

// isIdentifierByte reports whether a byte can be part of an identifier.
func isIdentifierByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		return true
	case c >= '0' && c <= '9', c == '_':
		return true
	case c >= 0x80:
		return true
	}
	return false
}

// exported reports whether a name is visible outside the package it is in.
func exported(name string) bool {
	for _, r := range name {
		return unicode.IsUpper(r)
	}
	return false
}

// trimTypeParams drops the type parameter list from a generic declaration,
// leaving the name it is declared under.
func trimTypeParams(name string) string {
	if bracket := strings.Index(name, "["); bracket >= 0 {
		return strings.TrimSpace(name[:bracket])
	}
	return name
}
