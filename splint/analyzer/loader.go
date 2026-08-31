package analyzer

import (
	"fmt"
	"go/ast"
	"log"
	"os"
	"path"
	"strings"

	"golang.org/x/tools/go/ast/inspector"

	"github.com/titpetric/tools/splint/analyzer/internal/collector"
	"github.com/titpetric/tools/splint/model"
)

// Load reads one package into its definitions.
//
// A file carrying a build tag is skipped: the loaded syntax is what the
// default build constraints selected, and a tagged file that came through with
// it would be read under constraints it was not written for.
func Load(in *Target, verbose bool) ([]*model.Definition, error) {
	if in == nil || in.Syntax == nil {
		return nil, fmt.Errorf("no syntax loaded for package: %s", in.Package)
	}

	pkg := in.Syntax
	sourcePath := in.Package.Path
	fset := pkg.Fset

	if verbose {
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
		facts = append(facts, fileFacts(fset, file, src))
	}

	sink := collector.NewCollector(fset)

	insp := inspector.New(files)
	insp.WithStack(nil, sink.Visit)

	results := sink.Clean(verbose)

	// Attach the package information to all returned definitions
	for _, def := range results {
		def.ImportPath = in.Package.ImportPath
		def.ID = in.Package.ID
		def.Files = facts
	}

	return results, nil
}
