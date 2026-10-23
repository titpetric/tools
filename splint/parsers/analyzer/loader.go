package analyzer

import (
	"fmt"
	"go/ast"
	"log"
	"os"
	"path"
	"strings"

	"golang.org/x/tools/go/ast/inspector"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/parsers/analyzer/internal/collector"
	"github.com/titpetric/tools/splint/pkg/splint"
)

// Load reads one package into its definitions.
//
// Load skips a file carrying a build tag: the loaded syntax is what the
// default build constraints selected, and a tagged file that came through with
// it would read under constraints its author never meant.
func Load(in *Target, options splint.Options) ([]*model.Definition, error) {
	if in == nil || in.Syntax == nil {
		return nil, fmt.Errorf("no syntax loaded for package: %s", in.Package)
	}

	pkg := in.Syntax
	sourcePath := in.Package.Path
	fset := pkg.Fset

	if options.Verbose {
		log.Printf("Loading package %s %q", sourcePath, in.Package.Name())
	}

	files := []*ast.File{}
	var facts model.FileList

	for _, file := range pkg.Syntax {
		filename := path.Base(fset.Position(file.Pos()).Filename)
		if !strings.HasSuffix(filename, ".go") {
			// skip test packages that don't end in .go
			continue
		}

		src, err := os.ReadFile(path.Join(sourcePath, filename))
		if err != nil {
			return nil, fmt.Errorf("Error reading in source file: %s", filename)
		}

		if tags := BuildTags(src); len(tags) > 0 {
			// skipped files with build tags
			continue
		}

		files = append(files, file)
		facts = append(facts, fileFacts(fset, file, src, options.IncludeImports))
	}

	// The loader requests types, so the collector resolves an identifier
	// against the package scope and never guesses at it.
	sink := collector.NewCollector(fset, pkg.TypesInfo, pkg.Types)

	insp := inspector.New(files)
	insp.WithStack(nil, sink.Visit)

	results := sink.Clean(options.Verbose)

	var excluded model.StringSet
	if options.IncludeTestFacts && !in.Package.TestPackage {
		var err error
		excluded, err = excludedTests(pkg.IgnoredFiles)
		if err != nil {
			return nil, err
		}
	}

	// Attach the package information to all returned definitions
	for _, def := range results {
		def.ImportPath = in.Package.ImportPath
		def.ID = in.Package.ID
		def.Files = facts
		def.Excluded = excluded
	}

	return results, nil
}

// excludedTests reads the test files the toolchain left out of a package and
// the constraint each carries. The loader lists a file left out for its name
// as well, handler_windows_test.go on linux, and that one writes no
// constraint, so it stays out of the set.
func excludedTests(ignored []string) (model.StringSet, error) {
	var excluded model.StringSet
	for _, filename := range ignored {
		name := path.Base(filename)
		if !strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		if expr := model.BuildConstraint(src); expr != "" {
			excluded.Add(name, expr)
		}
	}
	return excluded, nil
}
