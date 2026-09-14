package simpleparser

import (
	"slices"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// globals are the package level names a declaration reaches that its own file
// does not declare.
//
// The scan is every bare identifier of the lines the declaration covers, less
// what is written here: a name the file declares, a name the signature binds,
// a name the body binds, and the predeclared identifiers. What is left is
// either a package level name from another file or a local the scan did not
// see bound, and a reader of the field resolves it against the names the
// package declares.
//
// A name next to a dot is left out. It is a package qualifier or the field
// after one, and the qualifier of a package level variable is recorded by the
// reference split instead, which is where a name that is not an import goes.
func (f *file) globals(src *source, from, to int, shadowed []string) model.StringSet {
	var found model.StringSet

	locals := map[string]bool{}
	block := false

	record := func(text string) {
		eachIdentifier(text, func(name string) {
			if locals[name] || !f.reaches(name, shadowed) {
				return
			}
			found = add(found, name)
		})
	}

	for i := from; i <= to && i < src.len(); i++ {
		code := src.codeLine(i)

		// What the line binds is bound before the line is read, so a name is
		// a local from where it is declared: a body reaching a package level
		// name and then declaring a local of that name reaches both, and only
		// the first of them is a reference.
		bindLocals(locals, src, i, &block)

		// A struct or an interface written inside a body declares its fields
		// one per line, and the name it declares resolves against nothing.
		if typ := fieldLine(code, src.line(i)); typ != "" {
			f.recordSignature(typ, record)
			continue
		}

		record(code)
	}

	return found
}

// fieldLine is the type half of a line declaring a field, and is empty for a
// line that is a statement.
//
// A struct or an interface written inside a function body is the one place a
// body holds a declaration written as two words with nothing between them.
// Everything a statement is made of has something else on the line: an
// operator, a call, a block or a colon.
func fieldLine(code, written string) string {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" || strings.ContainsAny(trimmed, "=(){}:,") {
		return ""
	}

	name, typ, _ := splitField(trimmed, written)
	if name == "" || typ == "" || isKeyword(name) || !isIdentifier(name) {
		return ""
	}
	return typ
}

// recordSignature reads a type expression and records the names it resolves
// against, dropping the names it binds.
//
// A parameter list writes what it takes as well as what it is, and a method
// line writes the name it declares in front of it: "Handle(pattern string, h
// Handler)" names Handler and nothing else. A type with no parameter list is
// read whole.
func (f *file) recordSignature(text string, record func(string)) {
	// The first paren of a type expression opens the parameter list: nothing
	// can nest inside a list that has not opened yet.
	open := strings.Index(text, "(")
	if open < 0 {
		record(text)
		return
	}

	close := matchParen(text, open)
	if close < 0 {
		record(text)
		return
	}

	// What precedes the parens is the name being declared and the keyword in
	// front of it, and the name resolves against nothing.
	record(afterFirstWord(text[:open]))

	for _, group := range paramGroups(text[open+1 : close]) {
		f.recordSignature(group.Type, record)
	}
	for _, group := range resultGroups(strings.TrimSpace(text[close+1:])) {
		f.recordSignature(group.Type, record)
	}
}

// afterFirstWord drops the leading identifier of a signature head, which is
// the name it declares.
func afterFirstWord(head string) string {
	head = strings.TrimSpace(head)

	end := 0
	for end < len(head) && isIdentifierByte(head[end]) {
		end++
	}
	return strings.TrimSpace(head[end:])
}

// declGlobals are the package level names a type, const or var declaration
// reaches: the spec, and the body it opens where it has one.
//
// A struct field and an interface method write the name they declare first and
// what it is after it, and only the second half names anything the package
// resolves. A line the field split does not read that way is a value, and a
// value is read whole.
func (f *file) declGlobals(src *source, kind model.DeclarationKind, entries []entry) model.StringSet {
	var found model.StringSet

	record := func(text string) {
		eachIdentifier(text, func(name string) {
			if !f.reaches(name, nil) {
				return
			}
			found = add(found, name)
		})
	}

	// A type is written as types and the names it declares; a const and a var
	// are written as values, and a value is read whole.
	read := record
	if kind == model.TypeKind {
		read = func(text string) { f.recordSignature(text, record) }
	}

	for _, in := range entries {
		read(in.code)

		for i := in.open + 1; i <= in.close && i < src.len(); i++ {
			code := strings.TrimSpace(src.codeLine(i))
			if code == "" {
				continue
			}
			if typ := fieldLine(code, src.line(i)); typ != "" {
				read(typ)
				continue
			}
			read(code)
		}
	}

	return found
}

// reaches reports a name worth recording: one the file does not declare, the
// language does not declare, and the signature does not bind.
func (f *file) reaches(name string, shadowed []string) bool {
	return !f.names[name] && !predeclared[name] && !slices.Contains(shadowed, name)
}

// add records one name, keyed with no symbols: a bare identifier says which
// name was reached and nothing about what was read off it.
func add(set model.StringSet, name string) model.StringSet {
	if _, seen := set[name]; seen {
		return set
	}
	if set == nil {
		set = model.NewStringSet()
	}
	set.Add(name)
	return set
}

// eachIdentifier calls fn for every identifier written on its own on a line.
//
// A name with a dot on either side of it is skipped: what precedes a dot is a
// package qualifier or a value, and what follows one is a field or a method,
// and neither is a name resolved against the package.
func eachIdentifier(line string, fn func(name string)) {
	for i := 0; i < len(line); i++ {
		if !isIdentifierByte(line[i]) {
			continue
		}

		start := i
		for i < len(line) && isIdentifierByte(line[i]) {
			i++
		}

		name := line[start:i]
		if name[0] >= '0' && name[0] <= '9' {
			continue
		}
		// A dot on either side is a selector, and three of them are the
		// variadic marker: the type of "kind ...Kind" is written here as much
		// as the type of "kind Kind" is.
		if start > 0 && line[start-1] == '.' && !ellipsis(line, start-1) {
			continue
		}
		if i < len(line) && line[i] == '.' && !ellipsis(line, i) {
			continue
		}
		if compositeKey(line, start, i) {
			continue
		}

		fn(name)
	}
}

// ellipsis reports a dot that is part of the variadic marker, whichever of the
// three the index names.
func ellipsis(line string, at int) bool {
	for start := at - 2; start <= at; start++ {
		if start >= 0 && start+3 <= len(line) && line[start:start+3] == "..." {
			return true
		}
	}
	return false
}

// compositeKey reports the name on the left of a colon in a composite literal,
// which is the field it fills rather than a name resolved against anything.
//
// The position is what tells it from the other uses of a colon: a key opens an
// element, so what precedes it is the brace or the comma before it, or nothing
// at all on a literal written over several lines. A label and a switch case
// have a keyword or a statement in front of them, and an assignment has an
// equals sign after the colon.
//
// The key of a map literal is written in the same position and is read the same
// way, so a package level constant used as one is not recorded.
func compositeKey(line string, start, end int) bool {
	at := end
	for at < len(line) && line[at] == ' ' {
		at++
	}
	if at >= len(line) || line[at] != ':' {
		return false
	}
	if at+1 < len(line) && line[at+1] == '=' {
		return false
	}

	for start > 0 {
		start--
		switch line[start] {
		case ' ', '\t':
			continue
		case '{', ',':
			return true
		}
		return false
	}

	return true
}

// predeclared are the keywords and the identifiers the language declares in
// the universe scope. None of them is a name a package resolves, and leaving
// them in would put "err", "string" and "range" in the globals of everything.
var predeclared = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,

	"any": true, "bool": true, "byte": true, "comparable": true, "complex64": true,
	"complex128": true, "error": true, "float32": true, "float64": true, "int": true,
	"int8": true, "int16": true, "int32": true, "int64": true, "rune": true,
	"string": true, "uint": true, "uint8": true, "uint16": true, "uint32": true,
	"uint64": true, "uintptr": true,

	"append": true, "cap": true, "clear": true, "close": true, "complex": true,
	"copy": true, "delete": true, "imag": true, "len": true, "make": true,
	"max": true, "min": true, "new": true, "panic": true, "print": true,
	"println": true, "real": true, "recover": true,

	"false": true, "iota": true, "nil": true, "true": true, "_": true,
}
