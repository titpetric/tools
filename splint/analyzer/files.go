package analyzer

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// fileFacts reads what the model records about one file: the code in it, the
// bytes it takes, and whether anyone wrote it.
//
// The line count leaves out blanks and comments, which is what makes it a
// measure of the code rather than of the file. The simple parser counts the
// same lines the same way, and the parity harness compares the two.
func fileFacts(fset *token.FileSet, file *ast.File, source []byte) model.File {
	name := shortName(fset.Position(file.Pos()).Filename)

	return model.File{
		Name:      name,
		Lines:     codeLines(fset, file, strings.Split(string(source), "\n")),
		Size:      len(source),
		Generated: ast.IsGenerated(file),
		Test:      strings.HasSuffix(name, "_test.go"),
	}
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
