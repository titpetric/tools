package resolve_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/resolve"
)

// tree is two services that each hold a model package, a storage package under
// one of them, and one file that spells out an import the rest of the tree
// only reaches by name.
func tree() *model.DocumentRoot {
	module := &model.Module{
		Path: "example.com/app",
		Requires: []model.Require{
			{Path: "github.com/stretchr/testify", Version: "v1.11.1"},
			{Path: "gopkg.in/yaml.v3", Version: "v3.0.1"},
		},
	}

	pkg := func(dir, name string, files ...model.File) *model.Definition {
		importPath := module.Path
		if dir != "." {
			importPath += "/" + dir
		}
		return &model.Definition{
			Package: model.Package{
				Package:    name,
				ImportPath: importPath,
				Path:       "./" + dir,
			},
			Module: module,
			Files:  files,
		}
	}

	imports := func(name string, paths ...string) model.File {
		decl := model.ImportDecl{Block: true}
		for _, one := range paths {
			decl.Specs = append(decl.Specs, model.ImportSpec{Path: one})
		}
		return model.File{Name: name, ImportDecls: model.ImportDeclList{decl}}
	}

	root := model.NewDocumentRoot(".", "test")
	root.AddModule(module)
	root.Packages = model.DefinitionList{
		pkg(".", "app"),
		pkg("model", "model"),
		pkg("service1", "service1"),
		pkg("service1/model", "model"),
		pkg("service2", "service2"),
		pkg("service2/model", "model"),
		pkg("service2/storage", "storage"),
		pkg("client", "client", imports("client.go", "github.com/stretchr/testify/assert", "net/http")),
	}

	return root
}

// find returns the package of a directory, as the tests name it.
func find(root *model.DocumentRoot, dir string) *model.Definition {
	for _, def := range root.Packages {
		if def.Package.Path == "./"+dir {
			return def
		}
	}
	return nil
}

// TestLookupPrefersTheNearestPackage is the claim goimports cannot make: the
// same name means a different package in each service.
func TestLookupPrefersTheNearestPackage(t *testing.T) {
	root := tree()
	index := resolve.New(root, nil)

	tests := []struct {
		from   string
		name   string
		expect string
	}{
		{"service2/storage", "model", "example.com/app/service2/model"},
		{"service2", "model", "example.com/app/service2/model"},
		{"service1", "model", "example.com/app/service1/model"},
		{".", "model", "example.com/app/model"},
		{"client", "model", "example.com/app/model"},
	}

	for _, test := range tests {
		t.Run(test.from+" reaches "+test.name, func(t *testing.T) {
			result := index.Lookup(find(root, test.from), test.name)
			assert.Equal(t, test.expect, result.Path)
			assert.Equal(t, resolve.SourceLocal, result.Source)
		})
	}
}

// TestLookupLearnsFromTheTree covers a dependency placed by what another file
// already writes, with no module cache read and nothing asked of the network.
func TestLookupLearnsFromTheTree(t *testing.T) {
	root := tree()
	index := resolve.New(root, nil)

	result := index.Lookup(find(root, "service2/storage"), "assert")
	assert.Equal(t, "github.com/stretchr/testify/assert", result.Path)
	assert.Equal(t, resolve.SourceTree, result.Source)

	result = index.Lookup(find(root, "service1"), "http")
	assert.Equal(t, "net/http", result.Path)
	assert.Equal(t, resolve.SourceTree, result.Source)
}

// TestLookupFallsBackToTheModuleRequirement covers a name nothing in the tree
// writes yet, placed by the last segment of a requirement.
func TestLookupFallsBackToTheModuleRequirement(t *testing.T) {
	index := resolve.New(tree(), nil)

	result := index.Lookup(find(tree(), "service1"), "yaml")
	assert.Equal(t, "gopkg.in/yaml.v3", result.Path)
	assert.Equal(t, resolve.SourceModule, result.Source)
}

// TestLookupAliasWins covers the escape hatch: a name the configuration named
// resolves to what it says, whatever else answers to it.
func TestLookupAliasWins(t *testing.T) {
	root := tree()
	index := resolve.New(root, map[string]string{"model": "example.com/other/model"})

	result := index.Lookup(find(root, "service2/storage"), "model")
	assert.Equal(t, "example.com/other/model", result.Path)
	assert.Equal(t, resolve.SourceAlias, result.Source)
}

// TestLookupReportsAmbiguity covers two paths answering to one name: nothing
// is resolved and both are named, because a formatter guessing between them
// writes the wrong import half the time.
func TestLookupReportsAmbiguity(t *testing.T) {
	root := tree()
	root.Packages[7].Files[0].ImportDecls[0].Specs = append(
		root.Packages[7].Files[0].ImportDecls[0].Specs,
		model.ImportSpec{Path: "example.com/other/assert"},
	)

	result := resolve.New(root, nil).Lookup(find(root, "service1"), "assert")
	assert.False(t, result.Found())
	assert.True(t, result.Ambiguous())
	assert.Equal(t, []string{"example.com/other/assert", "github.com/stretchr/testify/assert"}, result.Candidates)
}

// TestLookupFindsNothing covers the name that has to be an error: no
// configuration names it, no package of the tree is called it, no file writes
// it and no requirement ends in it.
func TestLookupFindsNothing(t *testing.T) {
	root := tree()

	result := resolve.New(root, nil).Lookup(find(root, "service1"), "nope")
	assert.False(t, result.Found())
	assert.False(t, result.Ambiguous())
	assert.Empty(t, result.Source)
}

// TestLookupStopsAtTheModule covers a package of another module: it is reached
// by importing that module, so the walk up does not find it.
func TestLookupStopsAtTheModule(t *testing.T) {
	root := tree()
	nested := &model.Module{Path: "example.com/app/service2/tool"}
	root.AddModule(nested)
	root.Packages = append(root.Packages, &model.Definition{
		Package: model.Package{Package: "helper", ImportPath: "example.com/app/service2/tool/helper", Path: "./service2/tool/helper"},
		Module:  nested,
	})

	index := resolve.New(root, nil)

	assert.False(t, index.Lookup(find(root, "service2/storage"), "helper").Found(),
		"a package of the outer module does not reach into the nested one")
	assert.False(t, index.Lookup(find(root, "service2/tool/helper"), "model").Found(),
		"and the nested module does not reach back out for model either")
}
