package simpleparser

import (
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// file is one parsed file: its package clause, its imports, and the
// declarations it makes.
type file struct {
	// Package is the name in the package clause.
	Package string

	// Doc is the package comment, which is the doc comment above the package
	// clause.
	Doc string

	// Imports are the import literals, as the model records them: the path in
	// quotes, prefixed by the alias and a space when the file gives one.
	Imports []string

	// aliases is what each import is reached by in this file, which is what a
	// reference is recognised against.
	aliases map[string]bool

	// names are every name the file declares at the top level, types and
	// values and funcs alike. A selector reaching one of them reaches
	// something written here rather than a package.
	names map[string]bool

	// Types, Consts, Vars and Funcs are what the file declares.
	Types  model.DeclarationList
	Consts model.DeclarationList
	Vars   model.DeclarationList
	Funcs  model.DeclarationList
}

// scan reads one file.
//
// It walks the lines from the top and takes each top level construct in turn.
// Everything it looks for is anchored at column zero, which gofmt guarantees
// for every declaration and for the brace or paren that closes one: a
// construct is found by where it starts and ends rather than by balancing what
// is inside it.
func scan(src *source) *file {
	out := &file{aliases: map[string]bool{}, names: declaredNames(src)}

	for i := 0; i < src.len(); i++ {
		code := src.codeLine(i)
		if code == "" || code[0] == ' ' || code[0] == '\t' {
			continue
		}

		switch {
		case strings.HasPrefix(code, "package "):
			out.Package = strings.TrimSpace(strings.TrimPrefix(code, "package "))
			out.Doc = docAbove(src, i)

		case strings.HasPrefix(code, "import "), code == "import(":
			i = out.scanImports(src, i)

		case strings.HasPrefix(code, "func "), strings.HasPrefix(code, "func("):
			i = out.scanFunc(src, i)

		case strings.HasPrefix(code, "type "), code == "type(":
			i = out.scanGenDecl(src, i, model.TypeKind)

		case strings.HasPrefix(code, "const "), code == "const(":
			i = out.scanGenDecl(src, i, model.ConstKind)

		case strings.HasPrefix(code, "var "), code == "var(":
			i = out.scanGenDecl(src, i, model.VarKind)
		}
	}

	return out
}

// blockEnd returns the line holding the paren or brace that closes a
// declaration opened at line from, which is the first line at column zero
// holding nothing but that character.
//
// A construct that is never closed at column zero, which is a file that does
// not compile, ends where the file does: the parser reports what it read
// rather than failing, which is the whole point of reading this way.
func blockEnd(src *source, from int, closer byte) int {
	for i := from + 1; i < src.len(); i++ {
		code := strings.TrimRight(src.codeLine(i), " \t")
		if len(code) > 0 && code[0] == closer {
			return i
		}
	}
	return src.len() - 1
}

// docAbove returns the doc comment directly above a line, which is the
// unbroken run of comment lines that ends on the line before it.
//
// The markers are trimmed and the line breaks are kept, which is what
// go/ast's CommentGroup.Text does and therefore what the ast parser records.
// A reader wanting the paragraphs rejoined can do it; a reader wanting the
// lines back cannot.
func docAbove(src *source, line int) string {
	start := docStart(src, line)
	if start == line {
		return ""
	}

	lines := make([]string, 0, line-start)
	for i := start; i < line; i++ {
		text := trimMarker(src.line(i))
		if isDirective(src.line(i)) {
			continue
		}
		lines = append(lines, text)
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// docStart is the first line of the doc comment above a line, and the line
// itself when there is no comment above it.
func docStart(src *source, line int) int {
	start := line
	for start > 0 && src.isComment(start-1) {
		start--
	}
	return start
}

// trimMarker strips the comment markers from one line and returns what it
// says.
//
// Exactly one space after the marker is dropped and no more, which is what
// go/ast does and what keeps an indented code block in a doc comment indented:
// "//\tctx := ..." is an example a reader is meant to see as one.
func trimMarker(line string) string {
	text := strings.TrimLeft(line, " \t")
	switch {
	case strings.HasPrefix(text, "//"):
		text = text[2:]
		if strings.HasPrefix(text, " ") {
			text = text[1:]
		}
	case strings.HasPrefix(text, "/*"):
		text = strings.TrimSuffix(text[2:], "*/")
		if strings.HasPrefix(text, " ") {
			text = text[1:]
		}
	case strings.HasSuffix(text, "*/"):
		text = strings.TrimSuffix(text, "*/")
	case strings.HasPrefix(text, "*"):
		text = text[1:]
	}
	return strings.TrimRight(text, " \t")
}

// isDirective reports whether a comment is an instruction to a tool rather
// than something written for a reader: "//go:embed", "//line", "//export".
//
// go/ast leaves them out of a doc comment, so a document that carried them
// would say a symbol is documented with something nobody wrote for a reader.
// The rule is go/ast's: a lower case name, a colon, and something after it,
// with no space anywhere before the colon.
func isDirective(line string) bool {
	text := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(text, "//") {
		return false
	}
	text = text[2:]
	if len(text) < 2 || text[0] == ' ' {
		return false
	}
	if strings.HasPrefix(text, "line ") {
		return true
	}

	colon := strings.Index(text, ":")
	if colon <= 0 || colon+1 >= len(text) {
		return false
	}
	for i := 0; i <= colon+1; i++ {
		if i == colon {
			continue
		}
		if b := text[i]; !(b >= 'a' && b <= 'z' || b >= '0' && b <= '9') {
			return false
		}
	}
	return true
}

// declaredNames collects every name a file declares at the top level.
//
// go/parser resolves an identifier against the file it is written in, so a
// name declared here is a local to the collector and a name declared in
// another file of the same package is not. That is the line this draws, and
// it is why a package level var used across two files turns up in Globals.
func declaredNames(src *source) map[string]bool {
	names := map[string]bool{}

	for i := 0; i < src.len(); i++ {
		code := src.codeLine(i)
		if code == "" || code[0] == ' ' || code[0] == '\t' {
			continue
		}

		switch {
		case strings.HasPrefix(code, "func "), strings.HasPrefix(code, "func("):
			// Only the name is wanted, which is the word after "func" on a
			// plain declaration. A method declares nothing at package scope,
			// so a receiver means there is nothing to record.
			if name := funcName(code); name != "" {
				names[name] = true
			}

		case strings.HasPrefix(code, "type"), strings.HasPrefix(code, "const"), strings.HasPrefix(code, "var"):
			keyword, _, _ := strings.Cut(strings.TrimSpace(code), " ")
			for _, spec := range specsOf(src, i, keyword) {
				for _, name := range specNames(spec, keyword) {
					names[name] = true
				}
			}
		}
	}

	return names
}

// funcName is the name a func declaration opens with, and is empty for a
// method, which declares nothing at package scope.
func funcName(code string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(code, "func"))
	if rest == "" || rest[0] == '(' {
		return ""
	}

	end := 0
	for end < len(rest) && isIdentifierByte(rest[end]) {
		end++
	}
	if name := rest[:end]; isIdentifier(name) {
		return name
	}
	return ""
}

// specsOf returns the specs of a declaration, one for a single form and one
// per entry for a parenthesised block.
func specsOf(src *source, line int, keyword string) []string {
	code := strings.TrimSpace(src.codeLine(line))

	if strings.TrimSpace(strings.TrimSuffix(code, "(")) != keyword || !strings.HasSuffix(code, "(") {
		return []string{strings.TrimSpace(strings.TrimPrefix(code, keyword))}
	}

	var specs []string
	end := blockEnd(src, line, ')')
	for i := line + 1; i < end; i++ {
		if entry := strings.TrimSpace(src.codeLine(i)); entry != "" {
			specs = append(specs, entry)
			i = entryEnd(src, i)
		}
	}
	return specs
}

// specNames returns the names one spec declares.
func specNames(spec, keyword string) []string {
	if spec == "" {
		return nil
	}
	if keyword == "type" {
		name, _, _ := cutTypeName(spec)
		if name = trimTypeParams(name); name != "" {
			return []string{name}
		}
		return nil
	}
	return readValue(spec)
}

// bodyEnd returns the line closing a brace opened on line from, matched by
// depth rather than by column.
//
// blockEnd anchors on column zero, which is where gofmt puts the brace closing
// a top level declaration and nowhere else. A body nested inside one, an
// anonymous struct declared as the type of a field, closes at whatever
// indentation it was opened at, so its extent has to be counted.
func bodyEnd(src *source, from int) int {
	depth := 0

	for i := from; i < src.len(); i++ {
		code := src.codeLine(i)
		depth += strings.Count(code, "{") - strings.Count(code, "}")
		if i > from || strings.Contains(code, "{") {
			if depth <= 0 {
				return i
			}
		}
	}

	return src.len() - 1
}
