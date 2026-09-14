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
// A selector reaches a package when nothing in scope answers to the name: not
// a parameter, not a local of the body, and not something the file declares at
// the top level. That is the rule go/parser applies, and it is file scoped,
// which is why a package level var shared between two files is a reference in
// the file that does not declare it.
//
// What the file cannot decide is which of these are imports and which are the
// package's own, since that is settled against the import table of the whole
// package. splitGlobals does it once every file has been read.
//
// The shadowed names are the parameters and the receiver, which are locals
// whatever else they are called: "func Run(config config.Config)" reaches its
// argument in the body and not the package it is named after.
func (f *file) references(src *source, from, column, to int, shadowed []string) model.StringSet {
	var refs model.StringSet

	locals := bodyLocals(src, from, to)

	for i := from; i <= to && i < src.len(); i++ {
		// The body of a one line func starts partway along the line it is
		// declared on, and what precedes it is the signature: a type named
		// there is not something the body reached.
		code := src.codeLine(i)
		if i == from && column > 0 && column < len(code) {
			code = strings.Repeat(" ", column) + code[column:]
		}

		eachSelector(code, func(pkg, symbol string) {
			if pkg == "internal" || !exported(symbol) {
				return
			}
			// A name this file declares, or one the signature binds, is
			// something written here rather than a package reached from here.
			if f.names[pkg] || locals[pkg] || slices.Contains(shadowed, pkg) {
				return
			}
			if refs == nil {
				refs = model.NewStringSet()
			}
			if !slices.Contains(refs[pkg], symbol) {
				refs.Add(pkg, symbol)
			}
		})
	}

	return refs
}

// selector is one "pkg.Symbol" as it was written.
type selector struct {
	pkg    string
	symbol string
}

// eachSelector calls fn for every "left.Right" on a line.
//
// It yields rather than returning a list: the caller consumes each one and
// keeps none, so a slice per line of every body is a slice per line thrown
// away.
func eachSelector(line string, fn func(pkg, symbol string)) {
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

		fn(left, right)
		i = end
	}
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

	eachSelector(line, func(pkg, symbol string) {
		found = append(found, selector{pkg: pkg, symbol: symbol})
	})

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

// bodyLocals are the names a function body binds, which shadow whatever else
// they are called.
//
// It reads the two forms a body declares with, "x :=" and "var x", and the
// range clause, which is enough for the names that turn up on the left of a
// selector. A name it misses becomes a reference and then a global, which is
// where an unresolved name belongs.
func bodyLocals(src *source, from, to int) map[string]bool {
	locals := map[string]bool{}
	block := false

	for i := from; i <= to && i < src.len(); i++ {
		bindLocals(locals, src, i, &block)
	}

	return locals
}

// bindLocals records what one line binds.
//
// The block flag carries a var or const block open across the lines it covers,
// which is the one binding form that is not written on the line it applies to.
func bindLocals(locals map[string]bool, src *source, i int, block *bool) {
	code := src.codeLine(i)
	trimmedLine := strings.TrimSpace(code)

	// A var or const block inside a body binds one name per line.
	switch {
	case trimmedLine == "var (", trimmedLine == "const (":
		*block = true
		return
	case *block && strings.HasPrefix(trimmedLine, ")"):
		*block = false
		return
	case *block:
		spec := trimmedLine
		if eq := indexTop(spec, '='); eq >= 0 {
			spec = spec[:eq]
		}
		for _, part := range splitTop(spec, ',') {
			if fields := strings.Fields(part); len(fields) > 0 && isIdentifier(fields[0]) {
				locals[fields[0]] = true
			}
		}
		return
	}

	// "switch x := y.(type)" binds x in every case of the switch, which
	// reads as an assignment and is one.
	if assign := indexTop(code, ':'); assign >= 0 && assign+1 < len(code) && code[assign+1] == '=' {
		for _, name := range splitTop(strings.TrimSpace(code[:assign]), ',') {
			addLocal(locals, name)
		}
	}

	// A func literal binds its parameters for the length of its body,
	// which is where a callback's arguments come from.
	for _, open := range literalParams(code) {
		for _, group := range paramGroups(open) {
			for _, name := range group.Names {
				locals[name] = true
			}
		}
	}

	for _, keyword := range []string{"var ", "const "} {
		if !strings.HasPrefix(trimmedLine, keyword) {
			continue
		}
		spec := strings.TrimPrefix(trimmedLine, keyword)
		if eq := indexTop(spec, '='); eq >= 0 {
			spec = spec[:eq]
		}
		// A declaration writes the type after the names, so the first
		// word of each part is the name and the rest is what it is.
		for _, part := range splitTop(spec, ',') {
			if fields := strings.Fields(part); len(fields) > 0 && isIdentifier(fields[0]) {
				locals[fields[0]] = true
			}
		}
	}
}

// addLocal records one bound name.
//
// The name is the last word of what precedes the binding, since a statement
// binding one is often introduced by a keyword: "if info, err := os.Stat(x)"
// binds info and err, and "if" is not a name. A declaration writes the type
// after the name instead, "var out io.Writer", which the caller trims first.
func addLocal(locals map[string]bool, name string) {
	fields := strings.Fields(strings.TrimSpace(name))
	if len(fields) == 0 {
		return
	}
	if word := fields[len(fields)-1]; isIdentifier(word) {
		locals[word] = true
	}
}

// literalParams returns the parameter lists of every func literal on a line.
func literalParams(code string) []string {
	var lists []string

	for i := 0; i+5 <= len(code); i++ {
		if code[i:i+4] != "func" {
			continue
		}
		if i > 0 && isIdentifierByte(code[i-1]) {
			continue
		}
		open := i + 4
		for open < len(code) && code[open] == ' ' {
			open++
		}
		if open >= len(code) || code[open] != '(' {
			continue
		}
		close := matchParen(code, open)
		if close < 0 {
			continue
		}
		lists = append(lists, code[open+1:close])
		i = close
	}

	return lists
}
