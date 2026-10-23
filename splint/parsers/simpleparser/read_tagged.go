package simpleparser

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/pkg/splint"
)

// ReadTagged reads the test files a build constraint keeps out of a parsed
// document into its Tagged definitions, one test definition per directory.
//
// The parse of either parser records the names of those files under
// Definition.Excluded, and this reads them the way the simple parser reads
// any file: what they declare and reach, the facts a linter counting the
// tests of a file reads. Both parsers call it, so the two documents agree.
func ReadTagged(doc *model.DocumentRoot, options splint.Options) error {
	for _, def := range doc.Packages {
		if def == nil || len(def.Excluded) == 0 {
			continue
		}

		dir := filepath.Join(doc.Root, strings.TrimPrefix(strings.TrimPrefix(def.Package.Path, "."), "/"))
		tagged, err := readTagged(def, dir)
		if err != nil {
			return err
		}
		if tagged == nil {
			continue
		}
		if !options.IncludeSources {
			tagged.ClearSource()
		}
		doc.Tagged = append(doc.Tagged, tagged)
	}
	return nil
}

// readTagged reads the excluded test files of one directory into a test
// definition named for the package the definition is.
func readTagged(def *model.Definition, dir string) (*model.Definition, error) {
	importPath := strings.TrimSuffix(def.Package.ImportPath, "_test")
	out := &model.Definition{
		Package: model.Package{
			ID:          importPath + "_test",
			Package:     strings.TrimSuffix(def.Package.Package, "_test") + "_test",
			ImportPath:  importPath + "_test",
			Path:        def.Package.Path,
			TestPackage: true,
		},
		Module:   def.Module,
		Imports:  model.NewStringSet(),
		Excluded: model.NewStringSet(),
	}
	seen := map[string]bool{}

	for _, name := range def.Excluded.Keys() {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}

		src := newSource(name, data)
		parsed := scan(src)
		if parsed.Package == "" {
			continue
		}

		out.Excluded.Add(name, def.Excluded[name]...)
		out.Files = append(out.Files, model.File{
			Name:      name,
			Lines:     src.codeLines(),
			Size:      len(data),
			Generated: src.generated(),
			Test:      true,
		})
		merge(out, parsed, name, seen)
	}

	if len(out.Files) == 0 {
		return nil, nil
	}
	splitGlobals(out)
	out.Sort()
	return out, nil
}
