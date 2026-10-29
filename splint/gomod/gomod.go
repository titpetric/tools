package gomod

import (
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/mod/modfile"

	"github.com/titpetric/tools/splint/model"
)

// Read parses the go.mod at filename into the module facts the model
// carries: the go and toolchain directives and every pinned require.
func Read(filename string) (*model.Module, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	parsed, err := modfile.Parse(filename, data, nil)
	if err != nil {
		return nil, err
	}

	module := moduleFrom(parsed)

	// Read also reads the go.sum beside it when there is one. A module that
	// nobody has built yet has none, and a module is worth reporting on either
	// way.
	module.Sums, _ = ReadSum(filepath.Join(filepath.Dir(filename), "go.sum"))

	return module, nil
}

// Find returns the module governing dir, walking up from it to a go.mod. A
// tree with no go.mod returns no module and no error.
func Find(dir string) (*model.Module, error) {
	current, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	for {
		filename := filepath.Join(current, "go.mod")
		if _, err := os.Stat(filename); err == nil {
			return Read(filename)
		}

		parent := filepath.Dir(current)
		if parent == current {
			return nil, nil
		}
		current = parent
	}
}

// moduleFrom copies a parsed go.mod into the model.
//
// The require block sits in the order go mod tidy last left it, and a
// module is free to hold more than one, so moduleFrom sorts the lists by module
// path. Two revisions that require the same things then produce the same
// model, and a diff of the two reports nothing.
func moduleFrom(parsed *modfile.File) *model.Module {
	result := &model.Module{}

	if parsed.Module != nil {
		result.Path = parsed.Module.Mod.Path
	}
	if parsed.Go != nil {
		result.GoVersion = parsed.Go.Version
	}
	if parsed.Toolchain != nil {
		result.Toolchain = parsed.Toolchain.Name
	}

	for _, require := range parsed.Require {
		result.Requires = append(result.Requires, model.Require{
			Path:     require.Mod.Path,
			Version:  require.Mod.Version,
			Indirect: require.Indirect,
		})
	}
	sort.Slice(result.Requires, func(i, j int) bool {
		return result.Requires[i].Path < result.Requires[j].Path
	})

	for _, replace := range parsed.Replace {
		result.Replaces = append(result.Replaces, model.Replace{
			Path:       replace.Old.Path,
			Version:    replace.Old.Version,
			NewPath:    replace.New.Path,
			NewVersion: replace.New.Version,
		})
	}
	sort.Slice(result.Replaces, func(i, j int) bool {
		if result.Replaces[i].Path != result.Replaces[j].Path {
			return result.Replaces[i].Path < result.Replaces[j].Path
		}
		return result.Replaces[i].Version < result.Replaces[j].Version
	})

	return result
}
