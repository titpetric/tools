package simpleparser

import (
	"path"
	"strings"
)

// scanImports reads an import declaration, single or parenthesised, and
// returns the line it ended on.
//
// The literal is recorded the way the model records it: the path in quotes,
// and an alias in front of it separated by a space, but only when the alias
// says something the path does not. An alias that repeats the base name of the
// path is dropped, which is what the ast collector does with it.
func (f *file) scanImports(src *source, line int) int {
	if !strings.HasSuffix(strings.TrimSpace(src.codeLine(line)), "(") {
		f.addImport(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(src.line(line)), "import")))
		return line
	}

	end := blockEnd(src, line, ')')
	for i := line + 1; i < end; i++ {
		if entry := strings.TrimSpace(src.line(i)); entry != "" {
			f.addImport(entry)
		}
	}
	return end
}

// addImport records one import entry, as it is written inside the block.
func (f *file) addImport(entry string) {
	alias, quoted := splitImport(entry)
	if quoted == "" {
		return
	}

	clean := strings.Trim(quoted, `"`)
	name := path.Base(clean)

	literal := quoted
	switch alias {
	case "", name:
		// No alias, or one that repeats what the path already says.
	case "_":
		// A blank import is reached by no name at all.
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
