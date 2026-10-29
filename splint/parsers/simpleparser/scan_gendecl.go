package simpleparser

import (
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// scanGenDecl reads a type, const or var declaration and returns the line it
// ended on.
//
// A parenthesised block is one declaration, for any count of names
// it introduces: the parser records "const ( A = 1; B = 2 )" once under both names,
// with the block as its source and the comment above the keyword as its doc.
// That is what the ast parser records, because a block is one node to it.
func (f *file) scanGenDecl(src *source, line int, kind model.DeclarationKind) int {
	keyword := kind.String()

	entries, end := genDeclEntries(src, line, keyword)
	if len(entries) == 0 {
		return end
	}

	decl := &model.Declaration{
		Kind:   kind,
		File:   src.name,
		Line:   line + 1,
		Doc:    docAbove(src, line),
		Source: src.declText(docStart(src, line), end),
	}

	var names []string
	for _, entry := range entries {
		switch kind {
		case model.TypeKind:
			names = append(names, f.readType(decl, src, entry)...)
		default:
			names = append(names, readValue(entry.code)...)
		}
	}
	if len(names) == 0 {
		return end
	}

	decl.Globals = f.declGlobals(src, kind, entries)
	if len(names) == 1 {
		decl.Name = names[0]
	} else {
		decl.Names = names
		if kind == model.TypeKind {
			flattenFieldPaths(decl)
		}
	}

	switch kind {
	case model.TypeKind:
		f.Types = append(f.Types, decl)
	case model.ConstKind:
		f.Consts = append(f.Consts, decl)
	case model.VarKind:
		f.Vars = append(f.Vars, decl)
	}

	return end
}

// entry is one spec of a declaration: the line it opens on, the line its body
// closes on, and the text of it with the keyword removed.
type entry struct {
	open  int
	close int
	code  string
}

// genDeclEntries returns the specs of a declaration and the line the whole
// thing ended on. A single declaration is one entry; a parenthesised block is
// one per line that holds something.
func genDeclEntries(src *source, line int, keyword string) ([]entry, int) {
	code := strings.TrimSpace(src.codeLine(line))

	if !strings.HasSuffix(code, "(") || strings.TrimSpace(strings.TrimSuffix(code, "(")) != keyword {
		body := entryEnd(src, line)
		return []entry{{
			open:  line,
			close: body,
			code:  strings.TrimSpace(strings.TrimPrefix(code, keyword)),
		}}, body
	}

	end := blockEnd(src, line, ')')

	var entries []entry
	for i := line + 1; i < end; i++ {
		text := strings.TrimSpace(src.codeLine(i))
		if text == "" {
			continue
		}
		body := entryEnd(src, i)
		entries = append(entries, entry{open: i, close: body, code: text})
		i = body
	}

	return entries, end
}

// entryEnd is the line one spec ends on, which is past the line it opens on
// when it declares a struct, an interface or a parenthesised value.
func entryEnd(src *source, line int) int {
	// A value written as a raw string runs to wherever the string closes,
	// which the code on the line says nothing about: every brace and keyword
	// inside the string is text. The line reads as an opening brace or paren
	// once the string is blanked out, so this test comes first.
	if src.rawOpen(line) {
		end := line
		for src.rawOpen(end) && end < src.len()-1 {
			end++
		}
		return end
	}

	code := strings.TrimSpace(src.codeLine(line))
	switch {
	case strings.HasSuffix(code, "{"):
		// A composite literal closes more than once: "var x = []T{" ends on
		// the brace after the elements, and a line holding "}{" in between
		// closes the element type and opens the value. Counting depth finds
		// the last of them; a brace at column zero finds the first.
		return bodyEnd(src, line)
	case strings.HasSuffix(code, "("):
		return blockEnd(src, line, ')')
	}

	return line
}

// readType reads one type spec into the declaration and returns the name it
// declares.
//
// A block declaring two or more types folds into one declaration, so the fields
// accumulate and the last spec's type is the one recorded. That is what
// the ast collector does with the same input.
func (f *file) readType(decl *model.Declaration, src *source, in entry) []string {
	name, rest, found := cutTypeName(in.code)
	params := typeParams(name)
	name = trimTypeParams(strings.TrimSpace(name))
	if name == "" {
		return nil
	}
	if !found {
		return []string{name}
	}

	rest = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest), "{"))
	rest = strings.TrimPrefix(rest, "= ")

	switch {
	case rest == "struct" || strings.HasPrefix(rest, "struct{"), strings.HasPrefix(rest, "struct "):
		// A struct describes itself by its fields and records no type. Its
		// type parameters are the arguments the collector records, which it
		// reads off a struct alone.
		decl.Arguments = append(decl.Arguments, params...)
		decl.Fields = append(decl.Fields, structFields(src, in.open, in.close)...)
	case rest == "interface" || strings.HasPrefix(rest, "interface{"), strings.HasPrefix(rest, "interface "):
		decl.Type = "interface"
		decl.Fields = append(decl.Fields, interfaceMethods(src, in.open, in.close)...)
	default:
		decl.Type = symbolType(rest)
	}

	return []string{name}
}

// flattenFieldPaths drops the type prefix from the fields of a block
// declaring more than one type. The ast collector records such a block as one
// declaration with no single name, so its fields are known by the field name
// alone.
func flattenFieldPaths(decl *model.Declaration) {
	for _, field := range decl.Fields {
		if dot := strings.Index(field.Path, "."); dot >= 0 {
			field.Path = field.Path[dot+1:]
			continue
		}
		field.Path = ""
	}
}

// readValue reads one const or var spec and returns the names it declares.
//
// A spec names one symbol or more, and the last of them carries the type
// when the spec has one: "a, b int = 1, 2" declares two names of type int.
func readValue(code string) []string {
	head := code
	if eq := indexTop(head, '='); eq >= 0 {
		head = strings.TrimSpace(head[:eq])
	}
	if head == "" {
		return nil
	}

	names := splitTop(head, ',')
	if len(names) == 0 {
		return nil
	}

	// The type a value declares is not recorded: the ast collector reads a
	// type off a type declaration and off nothing else.
	last := names[len(names)-1]
	if space := indexTop(last, ' '); space >= 0 {
		names[len(names)-1] = strings.TrimSpace(last[:space])
	}

	out := make([]string, 0, len(names))
	for _, name := range names {
		if name = strings.TrimSpace(name); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// cutTypeName splits a type spec into the name it declares and the type
// behind that name.
//
// A generic type carries its parameters between the two, "List[T any] []T",
// and the space inside them is not the space that separates the halves.
func cutTypeName(spec string) (name, rest string, found bool) {
	spec = strings.TrimSpace(spec)

	if open := strings.Index(spec, "["); open > 0 && indexTop(spec[:open], ' ') < 0 {
		if close := matchBracket(spec, open); close > 0 {
			name = spec[:close+1]
			rest = strings.TrimSpace(spec[close+1:])
			return name, rest, rest != ""
		}
	}

	name, rest, found = strings.Cut(spec, " ")
	return name, strings.TrimSpace(rest), found
}

// typeParams reads the type parameters of a generic declaration, each as the
// name and the constraint that binds it. The model records these as the
// arguments of a type, the way a func records the types it takes.
func typeParams(name string) []string {
	open := strings.Index(name, "[")
	if open < 0 || !strings.HasSuffix(strings.TrimSpace(name), "]") {
		return nil
	}

	var params []string
	for _, group := range paramGroups(strings.TrimSuffix(strings.TrimSpace(name)[open+1:], "]")) {
		for _, param := range group.Names {
			params = append(params, param+" "+group.Type)
		}
	}
	return params
}
