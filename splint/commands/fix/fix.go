// Package fix rewrites the import block of every file that does not hold the
// one the house rule says it should.
//
// It is the only package here that writes a file. The rule lives in importfmt
// and the reporting of it in the imports linter, so what this writes is what
// the linter reports and nothing else: a run that fixes a tree clears exactly
// the findings a run that lints it produced.
package fix

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/titpetric/tools/splint/importfmt"
	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/resolve"
)

// Edit is a run of lines to replace, 1 indexed and inclusive of both ends.
//
// An edit with no text deletes the lines, which is what merging a second
// import declaration into the first one comes to.
type Edit struct {
	Line    int
	EndLine int
	Text    string
}

// FileFix is one file and what to do to it.
type FileFix struct {
	// Path is the file on disk, and Name is it as a report names it, relative
	// to the root of the parse.
	Path string
	Name string

	// Edits are the line runs to replace, in the order they appear in the
	// file.
	Edits []Edit

	// Added and Removed are what the rewrite changes, for a run that says
	// what it did.
	Added   []string
	Removed []string
}

// Skip is one file that needs a rewrite and cannot have one, and the names
// that stop it.
type Skip struct {
	// Name is the file as a report names it, relative to the root of the
	// parse.
	Name string

	// Unresolved are the names nothing could place, each with the symbols the
	// file reaches through it, so the report says which lines to look at.
	Unresolved []importfmt.Unresolved
}

// Plan is every file a run would rewrite.
type Plan struct {
	Files []FileFix

	// Skipped are the files that need a rewrite and cannot have one, because a
	// name in them does not resolve. Writing the block without that import
	// would leave a file that does not build.
	Skipped []Skip
}

// Len is how many files the plan rewrites.
func (p *Plan) Len() int {
	return len(p.Files)
}

// Build works out what every file of the tree should hold.
//
// Nothing is read from disk and nothing is written. The plan is a value the
// caller can print, count or hand to Apply.
func Build(root *model.DocumentRoot, opts importfmt.Options, aliases map[string]string) *Plan {
	plan := &Plan{}
	if root == nil {
		return plan
	}

	index := resolve.New(root, aliases)
	seen := map[string]bool{}

	for _, def := range root.Packages {
		decisions := importfmt.Decide(root, def, index, opts)

		for _, file := range def.Files {
			decision, known := decisions[file.Name]
			if !known {
				continue
			}

			// A file with a name nothing can place is reported whether or not
			// its block would otherwise change: the import it needs is
			// missing either way, and a run that said nothing about it would
			// read as a run that found nothing wrong.
			if decision.Sound() && !decision.Changed(file.ImportDecls) {
				continue
			}

			path := filepath.Join(root.Root, filepath.FromSlash(def.Package.Path), file.Name)
			if seen[path] {
				// A file is in the definition of its own scope and nowhere
				// else, but a document merged from two parses can hold it
				// twice, and rewriting it twice would apply the second set of
				// line numbers to the first rewrite's output.
				continue
			}
			seen[path] = true

			if !decision.Sound() {
				plan.Skipped = append(plan.Skipped, Skip{
					Name:       name(root, path),
					Unresolved: decision.Unresolved,
				})
				continue
			}

			plan.Files = append(plan.Files, FileFix{
				Path:    path,
				Name:    name(root, path),
				Edits:   edits(file.ImportDecls.Free(), decision.Text()),
				Added:   literals(decision.Added),
				Removed: literals(decision.Removed),
			})
		}
	}

	sort.Slice(plan.Files, func(i, j int) bool { return plan.Files[i].Path < plan.Files[j].Path })
	sort.Slice(plan.Skipped, func(i, j int) bool { return plan.Skipped[i].Name < plan.Skipped[j].Name })

	return plan
}

// edits are the line runs that turn a file's import declarations into one.
//
// The first declaration is replaced by the block and every later one is
// deleted, which is how two import declarations in a file become one. A file
// with no declaration at all gets one edit with no lines, which Apply reads as
// an insertion after the package clause.
func edits(decls model.ImportDeclList, text string) []Edit {
	if len(decls) == 0 {
		if text == "" {
			return nil
		}
		return []Edit{{Text: text}}
	}

	out := make([]Edit, 0, len(decls))
	out = append(out, Edit{Line: decls[0].Line, EndLine: decls[0].EndLine, Text: text})

	for _, decl := range decls[1:] {
		// The doc comment goes with the declaration it is about. It has been
		// carried into the block that replaces the first declaration, and
		// leaving it here would attach it to whatever follows.
		from := decl.Line
		if decl.DocLine > 0 && decl.DocLine < from {
			from = decl.DocLine
		}
		out = append(out, Edit{Line: from, EndLine: decl.EndLine})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Line < out[j].Line })

	return out
}

// literals are the specs as an import is written, for a run that says what it
// added or took away.
func literals(specs []model.ImportSpec) []string {
	if len(specs) == 0 {
		return nil
	}
	out := make([]string, 0, len(specs))
	for _, spec := range specs {
		out = append(out, spec.Literal())
	}
	return out
}

// name is a file as a report names it, relative to the root of the parse.
func name(root *model.DocumentRoot, path string) string {
	relative, err := filepath.Rel(root.Root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(relative)
}

// Apply writes the plan and returns the files it changed.
//
// A file whose rewrite comes to the bytes already on disk is not written and
// is not counted: a run over a tree that is already formatted changes nothing
// and says so.
func Apply(plan *Plan) ([]string, error) {
	var changed []string

	for _, file := range plan.Files {
		wrote, err := applyFile(file)
		if err != nil {
			return changed, err
		}
		if wrote {
			changed = append(changed, file.Name)
		}
	}

	return changed, nil
}

// applyFile rewrites one file, and reports whether anything changed.
func applyFile(file FileFix) (bool, error) {
	data, err := os.ReadFile(file.Path)
	if err != nil {
		return false, err
	}

	info, err := os.Stat(file.Path)
	if err != nil {
		return false, err
	}

	text := string(data)

	// A file written with CRLF keeps CRLF. The block is composed with newlines
	// alone, and writing it into a file that uses the other convention leaves
	// one file with both, which reads as a change to every line of it.
	crlf := strings.Contains(text, "\r\n")
	if crlf {
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}

	out, ok := Splice(text, file.Edits)
	if !ok || out == text {
		return false, nil
	}
	if crlf {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}

	return true, write(file.Path, []byte(out), info.Mode().Perm())
}

// Splice applies the edits to a file and returns the result, from the bottom
// up so an edit is applied to the line numbers the parse recorded rather than
// to the ones the edit before it left.
//
// It reports false when any edit does not land on an import declaration, and
// the file is then not written at all. The line range comes from a parse, and
// a parse that read the range wrong would have this overwrite whatever is
// actually there: a range that ran to the end of the file replaced the file
// with an import block. Checking what is about to be replaced costs one string
// comparison per edit and bounds the damage a parser bug can do to nothing.
func Splice(text string, edits []Edit) (string, bool) {
	lines := strings.Split(text, "\n")

	for i := len(edits) - 1; i >= 0; i-- {
		edit := edits[i]

		if edit.Line == 0 {
			lines = insert(lines, edit.Text)
			continue
		}

		from, to := edit.Line-1, edit.EndLine
		if from < 0 || to > len(lines) || from >= to {
			return text, false
		}
		if !importRange(lines[from:to]) {
			return text, false
		}

		var replacement []string
		if edit.Text != "" {
			replacement = strings.Split(edit.Text, "\n")
		} else if to < len(lines) && strings.TrimSpace(lines[to]) == "" {
			// The declaration is gone, and so is the blank line that used to
			// separate it from what follows. Without that a merge leaves two
			// blank lines where one declaration used to be.
			to++
		}

		lines = append(lines[:from], append(replacement, lines[to:]...)...)
	}

	return strings.Join(lines, "\n"), true
}

// importRange reports a run of lines that is an import declaration and nothing
// else: it opens on the import keyword, and a parenthesised one closes on the
// paren that matches.
func importRange(lines []string) bool {
	if len(lines) == 0 {
		return false
	}

	// A declaration may be reached through the comment lines above it, which
	// are deleted with it.
	at := 0
	for at < len(lines) {
		first := strings.TrimSpace(lines[at])
		if first != "" && !strings.HasPrefix(first, "//") && !strings.HasPrefix(first, "/*") {
			break
		}
		at++
	}
	if at >= len(lines) {
		return false
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[at]), "import") {
		return false
	}
	if at == len(lines)-1 {
		return true
	}

	return strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), ")")
}

// insert puts an import declaration into a file that has none, which is after
// the package clause and the blank line under it.
func insert(lines []string, text string) []string {
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "package ") {
			continue
		}

		at := i + 1
		block := append([]string{""}, strings.Split(text, "\n")...)
		return append(lines[:at], append(block, lines[at:]...)...)
	}

	return lines
}

// write replaces a file through a temporary beside it, so a run interrupted
// halfway leaves the file it found rather than half of one.
func write(path string, data []byte, mode os.FileMode) error {
	temp := path + ".splint-tmp"

	if err := os.WriteFile(temp, data, mode); err != nil {
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		os.Remove(temp)
		return err
	}

	return nil
}
