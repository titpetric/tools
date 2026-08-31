package simpleparser

import (
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// scanGenDecl reads a type, const or var declaration and returns the line it
// ended on.
//
// A parenthesised block is one declaration and not several, however many names
// it introduces: "const ( A = 1; B = 2 )" is recorded once under both names,
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
		Source: src.text(docStart(src, line), end),
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

	decl.SelfContained = f.selfContained(kind, entries)
	if len(names) == 1 {
		decl.Name = names[0]
	} else {
		decl.Names = names
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

	// fieldTypes are the types the body declares, which is what decides
	// whether the declaration reaches outside itself.
	fieldTypes []string
}

// genDeclEntries returns the specs of a declaration and the line the whole
// thing ended on. A single declaration is one entry; a parenthesised block is
// one per line that holds something.
func genDeclEntries(src *source, line int, keyword string) ([]entry, int) {
	code := strings.TrimSpace(src.codeLine(line))

	if !strings.HasSuffix(code, "(") || strings.TrimSpace(strings.TrimSuffix(code, "(")) != keyword {
		body := entryEnd(src, line)
		return []entry{{
			open:       line,
			close:      body,
			code:       strings.TrimSpace(strings.TrimPrefix(code, keyword)),
			fieldTypes: bodyTypes(src, line, body),
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
		entries = append(entries, entry{open: i, close: body, code: text, fieldTypes: bodyTypes(src, i, body)})
		i = body
	}

	return entries, end
}

// entryEnd is the line one spec ends on, which is past the line it opens on
// when it declares a struct, an interface or a parenthesised value.
func entryEnd(src *source, line int) int {
	code := strings.TrimSpace(src.codeLine(line))
	switch {
	case strings.HasSuffix(code, "{"):
		return blockEnd(src, line, '}')
	case strings.HasSuffix(code, "("):
		return blockEnd(src, line, ')')
	}
	return line
}

// readType reads one type spec into the declaration and returns the name it
// declares.
//
// A block declaring several types folds into one declaration, so the fields
// accumulate and the type of the last spec is the one recorded. That is what
// the ast collector does with the same input.
func (f *file) readType(decl *model.Declaration, src *source, in entry) []string {
	name, rest, found := strings.Cut(in.code, " ")
	decl.Arguments = append(decl.Arguments, typeParams(name)...)
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
		// A struct describes itself by its fields and records no type.
		decl.Fields = append(decl.Fields, structFields(src, in.open, in.close)...)
	case rest == "interface" || strings.HasPrefix(rest, "interface{"), strings.HasPrefix(rest, "interface "):
		decl.Type = "interface"
		decl.Fields = append(decl.Fields, interfaceMethods(src, in.open, in.close)...)
	default:
		decl.Type = symbolType(rest)
	}

	return []string{name}
}

// readValue reads one const or var spec and returns the names it declares.
//
// A spec names one symbol or several, and the last of them carries the type
// when the spec gives one: "a, b int = 1, 2" declares two names of type int.
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

// bodyTypes is the type of every field a body declares, which is what a self
// contained check reads.
func bodyTypes(src *source, open, close int) []string {
	if open >= close {
		return nil
	}

	var types []string
	for i := open + 1; i < close; i++ {
		code := strings.TrimSpace(src.codeLine(i))
		if code == "" {
			continue
		}
		if _, typ, _ := splitField(code, src.line(i)); typ != "" {
			types = append(types, typ)
		}
	}
	return types
}

// typeParams reads the type parameters of a generic declaration, each as the
// name and the constraint it is bound by. It is what the model records as the
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
