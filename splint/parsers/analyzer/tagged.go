package analyzer

import (
	"os"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/titpetric/tools/splint/model"
)

// loadTagged reads the test files a build constraint keeps out of the
// packages just read, typed the same as the rest: the directories holding
// one load a second time with the tags their constraints name, and the
// declarations of the excluded files go into one test definition per
// directory, the shape DocumentRoot.Tagged holds.
//
// A load that fails leaves the tagged files out; the document still holds
// their names under Definition.Excluded.
func (p *Parser) loadTagged(defs []*model.Definition) model.DefinitionList {
	excluded := map[string]*model.Definition{}
	var patterns []string
	tags := map[string]bool{}
	for _, def := range defs {
		if len(def.Excluded) == 0 || def.TestPackage {
			continue
		}
		if _, known := excluded[def.Package.Path]; known {
			continue
		}
		excluded[def.Package.Path] = def
		patterns = append(patterns, def.Package.Path)
		for _, exprs := range def.Excluded {
			for _, tag := range positiveTags(exprs) {
				tags[tag] = true
			}
		}
	}
	if len(patterns) == 0 || len(tags) == 0 {
		return nil
	}

	names := make([]string, 0, len(tags))
	for tag := range tags {
		names = append(names, tag)
	}
	cfg := &packages.Config{
		Mode:       packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedModule | packages.LoadSyntax,
		Tests:      true,
		BuildFlags: []string{"-tags=" + strings.Join(names, ",")},
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	byPath := map[string]*model.Definition{}
	var order []string
	for _, target := range cleanPackages(pkgs, cwd) {
		def, ok := excluded[target.Package.Path]
		if !ok || !target.Package.TestPackage {
			continue
		}
		loaded, err := Load(target, p.options)
		if err != nil {
			continue
		}

		out, known := byPath[def.Package.Path]
		if !known {
			out = taggedDefinition(def)
			byPath[def.Package.Path] = out
			order = append(order, def.Package.Path)
		}
		for _, in := range loaded {
			out.Merge(onlyFiles(in, def.Excluded))
		}
	}

	result := make(model.DefinitionList, 0, len(order))
	for _, path := range order {
		if def := byPath[path]; len(def.Files) > 0 {
			def.Sort()
			result = append(result, def)
		}
	}
	return result
}

// taggedDefinition is the empty test definition of a directory's tagged
// files, named for the package the definition is.
func taggedDefinition(def *model.Definition) *model.Definition {
	importPath := strings.TrimSuffix(def.Package.ImportPath, "_test")
	return &model.Definition{
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
}

// onlyFiles is the part of a loaded definition that the named files declare.
func onlyFiles(def *model.Definition, names model.StringSet) *model.Definition {
	keep := func(file string) bool {
		_, ok := names[file]
		return ok
	}
	out := &model.Definition{Imports: model.NewStringSet(), Excluded: model.NewStringSet()}
	for _, file := range def.Files {
		if keep(file.Name) {
			file.Test = true
			out.Files = append(out.Files, file)
			out.Excluded.Add(file.Name, names[file.Name]...)
		}
	}
	for file, imports := range def.Imports {
		if keep(file) {
			out.Imports.Add(file, imports...)
		}
	}
	inFiles := func(decl *model.Declaration) bool { return keep(decl.File) }
	out.Types = def.Types.Filter(inFiles)
	out.Consts = def.Consts.Filter(inFiles)
	out.Vars = def.Vars.Filter(inFiles)
	out.Funcs = def.Funcs.Filter(inFiles)
	return out
}

// positiveTags are the tags a set of constraints names without negating
// them, the ones a build sets to compile the files.
func positiveTags(exprs []string) []string {
	var out []string
	for _, expr := range exprs {
		words := strings.FieldsFunc(expr, func(r rune) bool {
			return r == ' ' || r == '(' || r == ')' || r == '&' || r == '|'
		})
		for _, word := range words {
			if word != "" && !strings.HasPrefix(word, "!") {
				out = append(out, word)
			}
		}
	}
	return out
}
