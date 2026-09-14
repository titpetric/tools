package simpleparser

import (
	"path"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// scanImports reads an import declaration, single or parenthesised, and
// returns the line it ended on.
//
// Two things are recorded from one walk. The literal is what the model has
// always carried: the path in quotes, and an alias in front of it separated by
// a space, but only when the alias says something the path does not. The
// declaration is the same imports as they are written, with their lines, their
// comments and the blank lines between them, which is what a formatter has to
// be able to put back.
func (f *file) scanImports(src *source, line int) int {
	code := src.codeLine(line)

	open := strings.IndexByte(code, '(')
	if open < 0 {
		decl := model.ImportDecl{Line: line + 1, EndLine: line + 1, Doc: docAbove(src, line)}
		if decl.Doc != "" {
			decl.DocLine = docStart(src, line) + 1
		}
		if spec, ok := f.addImport(src, line, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(src.line(line)), "import"))); ok {
			decl.Specs = append(decl.Specs, spec)
			f.ImportDecls = append(f.ImportDecls, decl)
		}
		return line
	}

	end := importBlockEnd(src, line, open)
	decl := model.ImportDecl{Line: line + 1, EndLine: end + 1, Block: true, Doc: docAbove(src, line)}
	if decl.Doc != "" {
		decl.DocLine = docStart(src, line) + 1
	}

	// A block written on one line holds its imports between the parens of that
	// line rather than on the lines below it.
	if end == line {
		for _, entry := range between(src, line, open) {
			spec, ok := f.addImport(src, line, entry)
			if !ok {
				continue
			}
			spec.Doc = docAbove(src, line)
			decl.Specs = append(decl.Specs, spec)
		}
		f.ImportDecls = append(f.ImportDecls, decl)
		return line
	}

	blank := false
	// consumed are the comment lines a spec took as its doc. What is left over
	// belongs to no import and is kept as the declaration's tail.
	consumed := map[int]bool{}

	for i := line + 1; i < end; i++ {
		entry := strings.TrimSpace(src.line(i))
		if entry == "" {
			blank = true
			continue
		}
		if src.isComment(i) {
			continue
		}

		spec, ok := f.addImport(src, i, entry)
		if !ok {
			continue
		}
		spec.Doc = docAbove(src, i)
		for at := docStart(src, i); at < i; at++ {
			consumed[at] = true
		}
		spec.Break = blank && len(decl.Specs) > 0
		blank = false
		decl.Specs = append(decl.Specs, spec)
	}

	decl.Tail = orphanComments(src, line, end, consumed)

	f.ImportDecls = append(f.ImportDecls, decl)
	return end
}

// orphanComments are the comment lines of a block that no import took as its
// doc, markers trimmed and in the order they were written.
func orphanComments(src *source, line, end int, consumed map[int]bool) string {
	var out []string

	for i := line + 1; i < end; i++ {
		if consumed[i] || !src.isComment(i) {
			continue
		}
		if isDirective(src.line(i)) {
			continue
		}
		out = append(out, trimMarker(src.line(i)))
	}

	return strings.TrimSpace(strings.Join(out, "\n"))
}

// importBlockEnd returns the line holding the paren that closes an import
// declaration opened at line, counting depth from the paren at open.
//
// blockEnd looks for a closer at column zero, which is where gofmt puts it.
// This does not: the tree a formatter is pointed at is the tree nobody has
// formatted, and a block whose closing paren is indented ends where the paren
// is rather than at the end of the file. Reading it as the end of the file is
// how a rewrite comes to replace the whole file with an import block.
func importBlockEnd(src *source, line, open int) int {
	depth := 0

	for i := line; i < src.len(); i++ {
		code := src.codeLine(i)
		from := 0
		if i == line {
			from = open
		}

		for at := from; at < len(code); at++ {
			switch code[at] {
			case '(':
				depth++
			case ')':
				if depth--; depth == 0 {
					return i
				}
			}
		}
	}

	// Nothing closed it, which is a file that does not compile. The
	// declaration is reported as the one line it opened on, so nothing
	// downstream replaces a range that reaches the end of the file.
	return line
}

// between are the entries a one line block holds between its parens, which a
// file writing more than one separates with a semicolon.
//
// The parens are found on the stripped view, where a paren inside a string is
// not one, and the entries are cut out of the line as it was written. Stripping
// blanks in place, so the two views share their columns.
func between(src *source, line, open int) []string {
	code := src.codeLine(line)

	close := strings.LastIndexByte(code, ')')
	if close <= open {
		return nil
	}

	raw := src.line(line)
	if close > len(raw) {
		return nil
	}

	var out []string
	for _, entry := range strings.Split(raw[open+1:close], ";") {
		if entry = strings.TrimSpace(entry); entry != "" {
			out = append(out, entry)
		}
	}

	return out
}

// addImport records one import entry, as it is written inside the block, and
// returns the spec it describes.
func (f *file) addImport(src *source, line int, entry string) (model.ImportSpec, bool) {
	alias, quoted := splitImport(entry)
	if quoted == "" {
		return model.ImportSpec{}, false
	}

	clean := strings.Trim(quoted, `"`)
	name := path.Base(clean)

	literal := quoted
	switch alias {
	case "", name:
		// No alias, or one that repeats what the path already says.
	case "_":
		// A blank import is reached by no name at all, and the underscore is
		// what says so: without it the literal reads as an import like any
		// other, and a rule about side effects has nothing to read.
		literal = "_ " + quoted
	case ".":
		// A dot import puts the package's names in this file's scope, which
		// is a thing worth recording even though no name reaches it.
		literal = ". " + quoted
	default:
		literal = alias + " " + quoted
		name = alias
	}

	f.Imports = append(f.Imports, literal)
	if alias != "_" && alias != "." {
		f.aliases[name] = true
	}

	// The comment is recorded the way a doc comment is, markers trimmed, which
	// is what the ast parser's CommentGroup.Text gives and therefore the only
	// form the two parsers can agree on.
	_, comment := src.split(line)
	if comment != "" {
		comment = strings.TrimSpace(trimMarker(comment))
	}

	return model.ImportSpec{
		Name:    alias,
		Path:    clean,
		Comment: comment,
		Line:    line + 1,
	}, true
}

// splitImport separates an import entry into its alias and its quoted path.
func splitImport(entry string) (alias, quoted string) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return "", ""
	}

	// A comment beside an import is not part of it.
	if marker := strings.Index(entry, "//"); marker >= 0 {
		entry = strings.TrimSpace(entry[:marker])
	}

	open := strings.Index(entry, `"`)
	if open < 0 {
		return "", ""
	}
	close := strings.LastIndex(entry, `"`)
	if close <= open {
		return "", ""
	}

	return strings.TrimSpace(entry[:open]), entry[open : close+1]
}
