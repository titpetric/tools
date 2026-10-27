package simpleparser

import (
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// scanFunc reads a func declaration and returns the line it ended on.
//
// A func opens at column zero and closes on a brace at column zero, which gofmt
// guarantees, and that makes the extent findable without balancing a
// single brace inside the body. A declaration with no body, the shape of a
// forward declaration to assembly, is one line.
func (f *file) scanFunc(src *source, line int) int {
	header, end, column := funcHeader(src, line)
	if header == "" {
		return line
	}

	// column is where the body opens on its header line, and is zero for a
	// declaration with no body at all: a forward declaration to assembly or a
	// go:linkname target. Scanning such a line for a closing brace would read
	// every declaration under it as the body, up to wherever some later brace
	// closes.
	body := end
	if column > 0 {
		body = bodyEndFrom(src, end, column-1)
	}

	decl := parseFuncHeader(header)
	if decl == nil {
		return body
	}

	decl.File = src.name
	decl.Line = line + 1
	decl.Doc = docAbove(src, line)
	decl.Source = src.declText(docStart(src, line), body)
	shadowed := localNames(header)
	if column > 0 {
		decl.References = f.references(src, end, column, body, shadowed)
	} else {
		decl.References = f.references(src, end+1, 0, body, shadowed)
	}
	// The globals cover the signature as well as the body: a parameter type
	// declared in another file is as much of a reach as a call to one. The
	// declared name sits there too, and a method declares one the
	// file does not record at package scope.
	decl.Globals = f.globals(src, line, body, append(shadowed, decl.Name))
	decl.Complexity = complexity(src, line, end+1, body)

	f.Funcs = append(f.Funcs, decl)

	return body
}

// funcHeader joins the lines of a signature that runs over more than one, and
// returns it with the line it ended on and the column the body opens at.
//
// A signature ends where its parens balance and the body brace follows the
// result type. Reading it as one string lets the split happen once and not per
// line. A struct or interface written as the result type carries a body of its
// own, which is kept as the source spells it: the ast collector prints the
// result with its line breaks and indentation, and the signature has to match.
func funcHeader(src *source, line int) (string, int, int) {
	// A signature that opens and closes on one line is that line. Most of them
	// are, and building a new string for each is most of what reading a func
	// costs.
	if code := src.codeLine(line); balanced(code) && !resultOpen(code) {
		if at := bodyBrace(code); at >= 0 {
			return strings.TrimSpace(code), line, at + 1
		}
		return strings.TrimSpace(code), line, 0
	}

	var (
		header strings.Builder
		depth  int
	)

	for i := line; i < src.len(); i++ {
		code := src.codeLine(i)
		indent := len(code) - len(strings.TrimLeft(code, " \t"))
		start, base, text := header.Len(), indent, strings.TrimSpace(code)

		if i > line {
			end := header.Len()
			if resultOpen(header.String()) {
				header.WriteString("\n")
				text, base = src.line(i), 0
			} else {
				header.WriteString(" ")
			}
			start = end + 1
		}
		header.WriteString(text)

		depth += strings.Count(code, "(") - strings.Count(code, ")")
		if depth > 0 {
			continue
		}

		joined := header.String()
		if resultOpen(joined) {
			continue
		}
		if at := bodyBrace(joined); at >= 0 {
			return strings.TrimSpace(joined), i, at - start + base + 1
		}

		trimmed := strings.TrimSpace(code)
		if strings.HasSuffix(trimmed, ",") {
			continue
		}
		if i > line || strings.Contains(code, ")") {
			return strings.TrimSpace(joined), i, 0
		}
	}

	return strings.TrimSpace(header.String()), src.len() - 1, 0
}

// parseFuncHeader reads a joined signature into a declaration.
func parseFuncHeader(header string) *model.Declaration {
	rest := strings.TrimSpace(strings.TrimPrefix(header, "func"))

	var receiver string
	if strings.HasPrefix(rest, "(") {
		close := matchParen(rest, 0)
		if close < 0 {
			return nil
		}
		if _, typ := splitParam(rest[1:close]); typ != "" {
			receiver = typ
		} else {
			receiver = strings.TrimSpace(rest[1:close])
		}
		rest = strings.TrimSpace(rest[close+1:])
	}

	open := strings.Index(rest, "(")
	if open < 0 {
		return nil
	}

	// A generic function carries its type parameters between the name and the
	// parameter list. They are not what the function takes, but the names they
	// bind still count: a result typed as one is not a type the package has.
	name := strings.TrimSpace(rest[:open])
	var typeParams []string
	if bracket := strings.Index(name, "["); bracket >= 0 {
		if close := matchBracket(rest, bracket); close > 0 {
			typeParams = typeParamNames(rest[bracket+1 : close])
		}
		name = strings.TrimSpace(name[:bracket])
	} else if strings.HasSuffix(name, "]") {
		if start := strings.Index(rest[:open], "["); start >= 0 {
			name = strings.TrimSpace(rest[:start])
		}
	}
	if name == "" {
		return nil
	}

	close := matchParen(rest, open)
	if close < 0 {
		return nil
	}

	params := paramGroups(rest[open+1 : close])
	results := resultGroups(trimBody(rest[close+1:]))

	return &model.Declaration{
		Kind:          model.FuncKind,
		Name:          name,
		TypeParams:    typeParams,
		Receiver:      symbolType(receiver),
		Arguments:     groupTypes(params),
		ArgumentCount: groupArity(params),
		Returns:       groupTypes(results),
		Signature:     signature(name, params, results),
	}
}

// typeParamNames are the names a type parameter list binds, read the way a
// parameter list is: the constraint is the group's type, and every name
// in front of the constraint is one parameter.
func typeParamNames(list string) []string {
	var names []string
	for _, group := range paramGroups(list) {
		names = append(names, group.Names...)
	}
	return names
}

// trimBody cuts the body off a result list, which is everything from the brace
// that opens it.
//
// A result may hold braces of its own, "func() struct{ A int }", so the body
// brace is the first one at depth zero that does not belong to a composite
// type. A signature with no body, which is a forward declaration to assembly,
// has nothing to cut.
func trimBody(results string) string {
	depth := 0

	for i := 0; i < len(results); i++ {
		switch results[i] {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case '{':
			if depth > 0 {
				continue
			}
			if opensComposite(results[:i]) {
				// The brace belongs to a struct or an interface in the result,
				// so the body is past what it closes.
				if end := matchBrace(results, i); end > 0 {
					i = end
					continue
				}
				return results
			}
			return strings.TrimSpace(results[:i])
		}
	}

	return strings.TrimSpace(results)
}

// opensComposite reports whether a brace follows a keyword that takes one,
// the test that tells a struct result from a function body.
func opensComposite(before string) bool {
	before = strings.TrimRight(before, " \t")
	return strings.HasSuffix(before, "struct") || strings.HasSuffix(before, "interface")
}

// matchBrace returns the index of the brace closing the one at open, and -1
// when nothing closes it.
func matchBrace(text string, open int) int {
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// localNames are the names a signature binds, which are the parameters, the
// results and the receiver. Every one of them shadows whatever else the name
// means, a package included.
func localNames(header string) []string {
	rest := strings.TrimSpace(strings.TrimPrefix(header, "func"))

	var names []string
	if strings.HasPrefix(rest, "(") {
		if close := matchParen(rest, 0); close > 0 {
			if name, typ := splitParam(rest[1:close]); typ != "" {
				names = append(names, name)
			}
			rest = strings.TrimSpace(rest[close+1:])
		}
	}

	open := strings.Index(rest, "(")
	if open < 0 {
		return names
	}
	close := matchParen(rest, open)
	if close < 0 {
		return names
	}

	for _, group := range paramGroups(rest[open+1 : close]) {
		names = append(names, group.Names...)
	}
	for _, group := range resultGroups(trimBody(rest[close+1:])) {
		names = append(names, group.Names...)
	}

	return names
}

// balanced reports whether a line opens and closes every paren on it, which is
// a signature that needs no joining.
func balanced(code string) bool {
	depth, seen := 0, false

	for i := 0; i < len(code); i++ {
		switch code[i] {
		case '(':
			depth++
			seen = true
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}

	return seen && depth == 0
}
