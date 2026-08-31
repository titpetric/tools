package analyzer

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/titpetric/tools/splint"
	"github.com/titpetric/tools/splint/model"
)

// ParserName is what a document says produced it.
const ParserName = "ast"

// Parser reads Go source through go/ast and x/tools, which is the exact
// reading: the toolchain resolves the packages, so what comes out is what the
// compiler sees.
//
// It is the slower of the two and the one that stops on source that does not
// build. Its opposite number is simpleparser, constructed the same way and
// returning the same document.
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

	defs, err := p.definitions(ctx, doc, root)
	if err != nil {
		return nil, err
	}

	doc.Packages = model.DefinitionList(defs)
	doc.Sort()
	return doc, nil
}

// definitions reads every package the options reach, recursively when the
// pattern says so and the one package in the source path when it does not.
func (p *Parser) definitions(ctx context.Context, doc *model.DocumentRoot, root string) ([]*model.Definition, error) {
	var defs []*model.Definition

	if p.options.Recursive() {
		modules, err := ListModules(p.options.SourcePath, p.options.Pattern)
		if err != nil {
			return nil, err
		}
		defs, err = p.walkModules(ctx, doc, root, modules)
		if err != nil {
			return nil, err
		}
	} else {
		// Resolved before walking: listing packages leaves the process in the
		// source directory, and a relative source path stops resolving to the
		// same place afterwards.
		module, err := FindModule(p.options.SourcePath)
		if err != nil {
			return nil, err
		}

		d, err := p.walkPackage(ctx, p.options.SourcePath)
		if err != nil {
			return nil, err
		}
		setModule(d, module)
		doc.AddModule(module)
		defs = d
	}

	defs = unique(defs)
	p.trim(defs)
	return defs, nil
}

// walkModules reads every module under the root, each rooted at its own go.mod
// so a package reports the module it actually builds under.
func (p *Parser) walkModules(ctx context.Context, doc *model.DocumentRoot, root string, modules []Module) ([]*model.Definition, error) {
	var result []*model.Definition

	for _, m := range modules {
		defs, err := p.walkPackage(ctx, m.Dir)
		if err != nil {
			return nil, err
		}

		// The go.mod was already located by ListModules, so there is nothing
		// left to search for; a module whose go.mod will not parse is reported
		// without one, the same as a tree that has none.
		if module, err := ReadModule(m.Filename); err == nil {
			setModule(defs, module)
			doc.AddModule(module)
		}

		// Paths are relative to the root of the parse, not to the module.
		if rel := strings.TrimPrefix(strings.TrimPrefix(m.Dir, root), string(filepath.Separator)); rel != "" {
			for _, def := range defs {
				def.Package.Path = "." + filepath.Join(rel, strings.TrimPrefix(def.Package.Path, "."))
			}
		}

		result = append(result, defs...)
	}
	return result, nil
}

// walkPackage reads every package under one source path.
func (p *Parser) walkPackage(ctx context.Context, sourcePath string) ([]*model.Definition, error) {
	defer runtime.GC()

	targets, err := ListPackages(sourcePath, p.options.Pattern)
	if err != nil {
		return nil, err
	}

	defs := []*model.Definition{}
	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		pkg := target.Package
		if pkg.TestPackage && !p.options.IncludeTests {
			continue
		}

		d, err := Load(target, p.options.Verbose)
		if err != nil {
			return nil, err
		}

		// A white box test is compiled into the package it tests, so the
		// listing names it the same. It is a different scope and is named as
		// one here.
		if pkg.TestPackage && !strings.HasSuffix(pkg.Package, "_test") {
			pkg.Package += "_test"
			pkg.ImportPath += "_test"
		}

		for _, def := range d {
			def.Package.ID = pkg.ID
			def.Package.ImportPath = pkg.ImportPath
			def.Package.Path = pkg.Path
			def.Package.Package = pkg.Package
			def.Package.TestPackage = pkg.TestPackage
		}

		defs = append(defs, d...)
		runtime.GC()
	}
	return defs, nil
}

// trim drops what the options did not ask for: the sources, and whichever half
// of the files the package is not.
func (p *Parser) trim(defs []*model.Definition) {
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

// setModule records the go.mod on every package extracted from it. A package
// that travels alone still has to say what it builds against.
func setModule(defs []*model.Definition, module *model.Module) {
	if module == nil {
		return
	}
	for _, def := range defs {
		def.Module = module
	}
}

// unique merges the definitions that name the same package, which is what the
// two halves of a package with a white box test arrive as.
func unique(defs []*model.Definition) []*model.Definition {
	result := make([]*model.Definition, 0, len(defs))

	for _, def := range defs {
		var match bool
		for _, res := range result {
			if res.ID == def.ID {
				match = true
				res.Merge(def)
				break
			}
		}
		if !match {
			result = append(result, def)
		}
	}

	return result
}
