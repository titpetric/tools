package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// fileFacts reads what the model records about one file: the code in it, the
// bytes it takes, and whether anyone wrote it.
//
// The line count leaves out blanks and comments, which is what makes it a
// measure of the code rather than of the file. The simple parser counts the
// same lines the same way, and the parity harness compares the two.
func fileFacts(fset *token.FileSet, file *ast.File, source []byte, imports bool) model.File {
	name := shortName(fset.Position(file.Pos()).Filename)
	lines := strings.Split(string(source), "\n")

	facts := model.File{
		Name:      name,
		Lines:     codeLines(fset, file, lines),
		Size:      len(source),
		Generated: ast.IsGenerated(file),
		Test:      strings.HasSuffix(name, "_test.go"),
	}

	if imports {
		facts.Package = file.Name.Name
		facts.Directives = model.HasLineDirectives(lines)
		facts.ImportDecls, facts.Uses, facts.Generated = importFacts(name, source, lines, file)
	}

	return facts
}

// importFacts reads the imports and the used names off the source as it is
// written, rather than off the syntax the package loader returned.
//
// A cgo file is not loaded as it was written. The toolchain preprocesses it
// before the loader sees it: import "C" is gone, an import of "unsafe" stands
// where it was, and the positions belong to a file that only exists inside the
// build. Writing an edit from those line numbers would replace the import of
// "C" with an import of "unsafe".
//
// Parsing the bytes again costs one ImportsOnly pass per file, which stops at
// the first declaration that is not an import. A file the loader did not
// rewrite parses to the same thing either way, so this is the reading for
// every file rather than a special case for one kind.
func importFacts(name string, source []byte, lines []string, loaded *ast.File) (model.ImportDeclList, []string, bool) {
	fset := token.NewFileSet()

	parsed, err := parser.ParseFile(fset, name, source, parser.ImportsOnly|parser.ParseComments)
	if err != nil {
		// The loader compiled it, so it parses; anything else is a file that
		// was read from disk differently than it was compiled from. Reporting
		// no imports leaves the file alone, which is what a reader that cannot
		// see it should do.
		return nil, nil, ast.IsGenerated(loaded)
	}

	decls := importDecls(fset, parsed, lines)

	// Whether a generator wrote the file is a fact about the file, and the
	// loaded syntax of a cgo file is what cmd/cgo wrote: it carries the
	// generated marker, and reading it there marks every cgo file as one
	// nobody wrote. The marker sits above the package clause, so the imports
	// only parse of the real bytes is enough to see it.
	generated := ast.IsGenerated(parsed)

	// The names a cgo file uses are read from the same bytes for the same
	// reason. Everything else is read from what the loader resolved.
	for _, decl := range decls {
		if !decl.Cgo() {
			continue
		}
		full, err := parser.ParseFile(fset, name, source, parser.ParseComments)
		if err != nil {
			return decls, fileUses(loaded), generated
		}
		return decls, fileUses(full), generated
	}

	return decls, fileUses(loaded), generated
}

// importDecls reads the import declarations of a file as they are written.
//
// The specs of a declaration carry their own doc and line comments, and the
// blank lines between them are read off the source: the syntax tree records
// where every spec begins, and the gap between one spec and the next is what
// says the file put them in different groups.
func importDecls(fset *token.FileSet, file *ast.File, lines []string) model.ImportDeclList {
	var decls model.ImportDeclList

	for _, node := range file.Decls {
		decl, ok := node.(*ast.GenDecl)
		if !ok || decl.Tok != token.IMPORT {
			continue
		}

		out := model.ImportDecl{
			Line:    fset.Position(decl.TokPos).Line,
			EndLine: fset.Position(decl.End()).Line,
			Block:   decl.Lparen.IsValid(),
			Doc:     commentText(decl.Doc),
		}
		if decl.Doc != nil {
			out.DocLine = fset.Position(decl.Doc.Pos()).Line
		}

		previous := 0
		for _, spec := range decl.Specs {
			imported, ok := spec.(*ast.ImportSpec)
			if !ok {
				continue
			}

			line := fset.Position(imported.Pos()).Line
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				path = strings.Trim(imported.Path.Value, `"`)
			}

			one := model.ImportSpec{
				Path:    path,
				Doc:     commentText(imported.Doc),
				Comment: commentText(imported.Comment),
				Line:    line,
			}
			if imported.Name != nil {
				one.Name = imported.Name.Name
			}
			if previous > 0 {
				one.Break = blankBetween(lines, previous, line-commentLines(imported.Doc))
			}
			previous = line

			out.Specs = append(out.Specs, one)
		}

		out.Tail = orphanComments(fset, file, decl)
		decls = append(decls, out)
	}

	return decls
}

// orphanComments are the comment lines inside an import declaration that no
// spec took as its doc or its line comment.
//
// A comment separated from the import below it by a blank line, and one after
// the last import, belong to the declaration and to no import in it. A rewrite
// that dropped them would be deleting what somebody wrote.
func orphanComments(fset *token.FileSet, file *ast.File, decl *ast.GenDecl) string {
	if !decl.Lparen.IsValid() {
		return ""
	}

	taken := map[*ast.CommentGroup]bool{decl.Doc: true}
	for _, spec := range decl.Specs {
		imported, ok := spec.(*ast.ImportSpec)
		if !ok {
			continue
		}
		taken[imported.Doc] = true
		taken[imported.Comment] = true
	}

	var out []string
	for _, group := range file.Comments {
		if taken[group] || group.Pos() < decl.Lparen || group.End() > decl.End() {
			continue
		}
		if text := strings.TrimSpace(group.Text()); text != "" {
			out = append(out, text)
		}
	}

	return strings.Join(out, "\n")
}

// commentText is a comment group as the model records it: the markers trimmed
// and the trailing newline taken off, which is what go/ast's Text does.
func commentText(group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}
	return strings.TrimSpace(group.Text())
}

// commentLines is how many lines a comment group occupies, which is how far
// above a spec its own doc reaches.
func commentLines(group *ast.CommentGroup) int {
	if group == nil {
		return 0
	}
	return len(group.List)
}

// blankBetween reports a blank line between two lines of a file, exclusive of
// both.
func blankBetween(lines []string, from, to int) bool {
	for line := from + 1; line < to; line++ {
		if blank(lines, line) {
			return true
		}
	}
	return false
}

// fileUses are the identifiers a file writes before a dot, sorted and
// deduplicated.
//
// Everything the tree calls a selector counts, so a field access on a local
// variable is in here alongside a package reference. The set answers whether
// removing an import would break the file, and keeping one too many is the
// only error that cannot.
func fileUses(file *ast.File) []string {
	seen := map[string]bool{}

	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if ident, ok := selector.X.(*ast.Ident); ok {
			seen[ident.Name] = true
		}
		return true
	})

	if len(seen) == 0 {
		return nil
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

// shortName is the base filename, which is what a declaration records.
func shortName(filename string) string {
	if at := strings.LastIndexByte(filename, '/'); at >= 0 {
		return filename[at+1:]
	}
	return filename
}

// codeLines counts the lines of a file that carry code, so blank lines and the
// lines a comment owns are left out.
//
// A comment owns a line when nothing but whitespace precedes it and nothing
// follows what it closes. A line carrying code and a comment beside it is a
// line of code: dropping it would count "x := 1 // why" as documentation.
func codeLines(fset *token.FileSet, file *ast.File, lines []string) int {
	owned := make(map[int]bool)

	for _, group := range file.Comments {
		from := fset.Position(group.Pos())
		to := fset.Position(group.End())

		// The line it opens on is the comment's only if nothing precedes it.
		if nothingBefore(lines, from.Line, from.Column) {
			owned[from.Line] = true
		}
		// Every line between is inside the comment.
		for line := from.Line + 1; line < to.Line; line++ {
			owned[line] = true
		}
		// The line it closes on is the comment's only if nothing follows it.
		if to.Line > from.Line && nothingAfter(lines, to.Line, to.Column) {
			owned[to.Line] = true
		}
	}

	count := 0
	for line := fset.Position(file.Pos()).Line; line <= fset.Position(file.End()).Line; line++ {
		if owned[line] || blank(lines, line) {
			continue
		}
		count++
	}

	return count
}

// nothingBefore reports a line holding only whitespace up to a column.
func nothingBefore(lines []string, line, column int) bool {
	text := lineAt(lines, line)
	if column-1 > len(text) {
		return true
	}
	return strings.TrimSpace(text[:column-1]) == ""
}

// nothingAfter reports a line holding only whitespace from a column on.
func nothingAfter(lines []string, line, column int) bool {
	text := lineAt(lines, line)
	if column-1 >= len(text) {
		return true
	}
	return strings.TrimSpace(text[column-1:]) == ""
}

// blank reports a line holding nothing but whitespace.
func blank(lines []string, line int) bool {
	return strings.TrimSpace(lineAt(lines, line)) == ""
}

// lineAt returns one line of a file, and nothing past its end.
func lineAt(lines []string, line int) string {
	if line < 1 || line > len(lines) {
		return ""
	}
	return lines[line-1]
}
