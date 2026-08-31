package simpleparser

import (
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// scanFunc reads a func declaration and returns the line it ended on.
//
// A func opens at column zero and closes on a brace at column zero, which gofmt
// guarantees and which is what makes the extent findable without balancing a
// single brace inside the body. A declaration with no body, which is what a
// forward declaration to assembly looks like, is one line.
func (f *file) scanFunc(src *source, line int) int {
	header, end := funcHeader(src, line)
	if header == "" {
		return line
	}

	// A body written on the same line as the signature closes on that line;
	// anything else closes on a brace at column zero. Where the body starts is
	// what keeps the signature out of the references.
	body, column := end, 0
	if closedOnLine(src.codeLine(end)) {
		column = bodyColumn(src.codeLine(end))
	} else {
		body = blockEnd(src, end, '}')
	}

	decl := parseFuncHeader(header)
	if decl == nil {
		return body
	}

	decl.File = src.name
	decl.Line = line + 1
	decl.Doc = docAbove(src, line)
	decl.Source = src.text(docStart(src, line), body)
	shadowed := localNames(header)
	if column > 0 {
		decl.References = f.references(src, end, column, body, shadowed)
	} else {
		decl.References = f.references(src, end+1, 0, body, shadowed)
	}
	decl.Complexity = complexity(src, end+1, body, decl.Source)

	if decl.Name == "init" && decl.Receiver == "" {
		f.InitCount++
	}
	f.Funcs = append(f.Funcs, decl)

	return body
}

// funcHeader joins the lines of a signature that runs over more than one, and
// returns it with the line it ended on.
//
// A signature ends where its parens balance and the line ends in an opening
// brace or in the result type. Reading it as one string is what lets the
// splitting be done once rather than per line.
func funcHeader(src *source, line int) (string, int) {
	var (
		header strings.Builder
		depth  int
	)

	for i := line; i < src.len(); i++ {
		code := src.codeLine(i)
		if i > line {
			header.WriteString(" ")
		}
		header.WriteString(strings.TrimSpace(code))

		depth += strings.Count(code, "(") - strings.Count(code, ")")
		if depth > 0 {
			continue
		}

		trimmed := strings.TrimSpace(code)
		if strings.HasSuffix(trimmed, ",") {
			continue
		}
		if strings.HasSuffix(trimmed, "{") || i > line || strings.Contains(code, ")") {
			return strings.TrimSpace(header.String()), i
		}
	}

	return strings.TrimSpace(header.String()), src.len() - 1
}

// closedOnLine reports whether a body opened on a line also closes on it,
// which is what a one line func does.
func closedOnLine(code string) bool {
	depth := 0
	opened := false

	for i := 0; i < len(code); i++ {
		switch code[i] {
		case '{':
			depth++
			opened = true
		case '}':
			depth--
		}
	}

	return opened && depth == 0
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
	// parameter list. They are not what the function takes.
	name := strings.TrimSpace(rest[:open])
	if bracket := strings.Index(name, "["); bracket >= 0 {
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
		Kind:      model.FuncKind,
		Name:      name,
		Receiver:  symbolType(receiver),
		Arguments: groupTypes(params),
		Returns:   groupTypes(results),
		Signature: signature(name, params, results),
	}
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
// which is what tells a struct result from a function body.
func opensComposite(before string) bool {
	before = strings.TrimRight(before, " \t")
	return strings.HasSuffix(before, "struct") || strings.HasSuffix(before, "interface")
}

// matchBrace returns the index of the brace closing the one at open, and -1
// when it is never closed.
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

// bodyColumn is where the body opens on a line that holds the whole
// declaration, which is the brace that a matching one closes at the end of it.
func bodyColumn(code string) int {
	depth := 0

	for i := 0; i < len(code); i++ {
		switch code[i] {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case '{':
			if depth > 0 {
				continue
			}
			if opensComposite(code[:i]) {
				if end := matchBrace(code, i); end > 0 {
					i = end
				}
				continue
			}
			return i + 1
		}
	}

	return 0
}

// localNames are the names a signature binds, which are the parameters, the
// results and the receiver. Every one of them shadows whatever else it is
// called, a package included.
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
