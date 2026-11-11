// Package yamlprose reads the prose out of a YAML file.
//
// A configuration file holds writing. A comment explains a decision. A field
// like desc, help or summary states a sentence somebody reads in a help page
// or a report. Both answer to the rules a doc comment answers to. No
// linter has read either.
//
// The reader walks the YAML tree and matches no pattern. The file's own shape
// decides what a comment attaches to. A quoted value holding a # stays a
// value. No schema reaches this package: a key is a key.
package yamlprose

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	"github.com/titpetric/tools/splint/model"
)

// The trees whose configuration belongs to somebody else.
var skipDirs = map[string]bool{
	"vendor":       true,
	"node_modules": true,
	".git":         true,
	"testdata":     true,
}

// Prose reads the comments and the sentence-shaped values of every YAML file in a tree.
func Prose(root string, recursive bool) ([]*model.ProseFile, error) {
	files, err := Files(root, recursive)
	if err != nil {
		return nil, err
	}

	var found []*model.ProseFile
	for _, path := range files {
		file, err := ProseFile(root, path)
		if err != nil {
			return nil, err
		}
		if file != nil && len(file.Blocks) > 0 {
			found = append(found, file)
		}
	}

	return found, nil
}

// ProseFile reads one file's prose, and answers no blocks and no error for a file it cannot parse.
func ProseFile(root, path string) (*model.ProseFile, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	name := path
	if rel, relErr := filepath.Rel(root, path); relErr == nil {
		name = filepath.ToSlash(rel)
	}

	file := &model.ProseFile{Path: name}

	parsed, err := parser.ParseBytes(source, parser.ParseComments)
	if err != nil {
		return file, nil
	}

	for _, doc := range parsed.Docs {
		if doc.Body == nil {
			continue
		}
		walk(file, doc.Body, 0)
	}

	return file, nil
}

// A mapping and its own entries are one level; the value under an entry is the next.
func walk(file *model.ProseFile, node ast.Node, depth int) {
	if node == nil {
		return
	}

	comment(file, node, depth)

	switch typed := node.(type) {
	case *ast.MappingNode:
		for _, value := range typed.Values {
			walk(file, value, depth)
		}
	case *ast.MappingValueNode:
		comment(file, typed.Key, depth)
		walk(file, typed.Value, depth+1)
	case *ast.SequenceNode:
		for _, value := range typed.Values {
			walk(file, value, depth)
		}
	case *ast.DocumentNode:
		walk(file, typed.Body, depth)
	case *ast.StringNode:
		sentence(file, typed.GetToken().Position.Line, typed.Value)
	case *ast.LiteralNode:
		if typed.Value != nil {
			sentence(file, typed.Value.GetToken().Position.Line, typed.Value.Value)
		}
	}
}

// A contiguous run of # lines is one group, which is the block every rule measures.
func comment(file *model.ProseFile, node ast.Node, depth int) {
	if node == nil {
		return
	}

	group := node.GetComment()
	if group == nil {
		return
	}

	var (
		lines []string
		line  int
	)

	// A bare # ends a paragraph, the way a blank line ends one in markdown.
	flush := func() {
		add(file, model.ProseBlock{Line: line, Comment: true, Depth: depth}, strings.Join(lines, " "))
		lines, line = nil, 0
	}

	for _, one := range group.Comments {
		if one == nil {
			continue
		}

		text := strings.TrimSpace(one.Token.Value)

		// A shebang is a comment to YAML and an instruction to the kernel.
		if text == "" || strings.HasPrefix(text, "!") {
			flush()
			continue
		}

		if line == 0 {
			line = one.GetToken().Position.Line
		}
		lines = append(lines, text)
	}

	flush()
}

// A path, a version and a shell command are data no rule about writing judges.
func sentence(file *model.ProseFile, line int, value string) {
	if !Sentence(value) {
		return
	}
	add(file, model.ProseBlock{Line: line}, value)
}

func add(file *model.ProseFile, block model.ProseBlock, text string) {
	block.Text = strings.Join(strings.Fields(text), " ")
	if block.Text == "" || block.Line == 0 {
		return
	}

	file.Blocks = append(file.Blocks, block)
}

// Files returns the YAML files of a tree, in the order the walk reaches them.
// A root naming one file answers that file alone.
func Files(root string, recursive bool) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		if !isYAML(root) {
			return nil, nil
		}
		return []string{root}, nil
	}

	var found []string
	walkDir := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			switch {
			case path == root:
				return nil
			case skipDirs[entry.Name()], entry.Name()[0] == '.':
				return fs.SkipDir
			case !recursive:
				return fs.SkipDir
			}
			return nil
		}

		if isYAML(path) {
			found = append(found, path)
		}
		return nil
	}

	if err := filepath.WalkDir(root, walkDir); err != nil {
		return nil, err
	}

	return found, nil
}

// isYAML reports whether a path names a YAML file, by either spelling.
func isYAML(path string) bool {
	switch filepath.Ext(path) {
	case ".yml", ".yaml":
		return true
	}
	return false
}
