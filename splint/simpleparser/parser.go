package simpleparser

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/titpetric/tools/splint"
	"github.com/titpetric/tools/splint/model"
)

// ParserName is what a document says produced it.
const ParserName = "simple"

// Parser reads Go source without building a syntax tree.
//
// It reads lines and matches the constructs by where they start and where they
// end, which gofmt puts at column zero. That buys two things the ast parser
// cannot give: it does not need the tree to resolve, so it reads source that
// does not compile, and it does not build one, so it is quick.
//
// It is constructed the same way as the ast parser and returns the same
// document, so a program swaps one for the other by changing an import.
type Parser struct {
	options splint.Options
}

// New returns a parser for the options given.
func New(options splint.Options) *Parser {
	return &Parser{options: options}
}

// Parse reads the tree and returns the document.
func (p *Parser) Parse(ctx context.Context) (*model.DocumentRoot, error) {
	root, err := filepath.Abs(p.options.SourcePath)
	if err != nil {
		return nil, err
	}

	doc := model.NewDocumentRoot(root, ParserName)

	dirs, err := p.directories(root)
	if err != nil {
		return nil, err
	}

	modules := newModuleTree(root)
	for _, dir := range dirs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		defs, err := p.readDir(root, modules.dirOf(dir), dir)
		if err != nil {
			return nil, err
		}
		for _, def := range defs {
			if module := modules.of(dir); module != nil {
				def.Module = module
				doc.AddModule(module)
			}
			doc.Packages = append(doc.Packages, def)
		}
	}

	p.trim(doc.Packages)
	doc.Sort()
	return doc, nil
}

// directories returns the directories holding Go files, which is one per
// package. A non recursive parse reads the source path alone.
func (p *Parser) directories(root string) ([]string, error) {
	if !p.options.Recursive() {
		return []string{root}, nil
	}

	var dirs []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		switch name := entry.Name(); {
		case path == root:
		case name == "vendor", name == "testdata", name == "node_modules":
			return filepath.SkipDir
		case strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_"):
			return filepath.SkipDir
		}
		if hasGoFiles(path) {
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(dirs)
	return dirs, nil
}

// hasGoFiles reports whether a directory holds a Go file of its own.
func hasGoFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			return true
		}
	}
	return false
}

// trim drops what the options did not ask for.
func (p *Parser) trim(defs model.DefinitionList) {
	for _, def := range defs {
		if !p.options.IncludeSources {
			def.ClearSource()
		}
		if !def.TestPackage || !p.options.IncludeTests {
			def.ClearTestFiles()
		}
		if def.TestPackage {
			def.ClearNonTestFiles()
		}
	}
}
