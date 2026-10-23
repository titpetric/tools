package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/titpetric/tools/splint/model"
)

// tagged is a directory with a test file in the build and two kept out: one
// behind the integration tag, one behind a platform.
func tagged() *model.DocumentRoot {
	return &model.DocumentRoot{
		Packages: model.DefinitionList{
			{
				Package: model.Package{Package: "store", Path: "./store"},
				Files:   model.FileList{{Name: "store.go"}},
			},
			{
				Package: model.Package{Package: "store_test", Path: "./store", TestPackage: true},
				Files:   model.FileList{{Name: "unit_test.go", Test: true}},
				Funcs:   model.DeclarationList{{Kind: model.FuncKind, Name: "TestUnit", File: "unit_test.go"}},
			},
		},
		Tagged: model.DefinitionList{{
			Package:  model.Package{Package: "store_test", Path: "./store", TestPackage: true},
			Files:    model.FileList{{Name: "store_test.go", Test: true}, {Name: "linux_test.go", Test: true}},
			Excluded: model.StringSet{"store_test.go": {"integration"}, "linux_test.go": {"linux"}},
			Funcs: model.DeclarationList{
				{Kind: model.FuncKind, Name: "TestOpen", File: "store_test.go"},
				{Kind: model.FuncKind, Name: "TestLinux", File: "linux_test.go"},
			},
		}},
	}
}

func TestDocumentRoot_WithTagged(t *testing.T) {
	t.Parallel()

	in := tagged()
	out := in.WithTagged("integration")

	require.Len(t, out.Packages, 2)
	tests := out.Packages[1]
	assert.ElementsMatch(t, []string{"store_test.go", "unit_test.go"}, fileNames(tests.Files))
	assert.Len(t, tests.Funcs, 2, "the test behind the tag joins, the one behind linux does not")
	assert.Empty(t, out.Tagged)

	assert.Len(t, in.Packages[1].Files, 1, "the document read stays as it was")
	assert.Len(t, in.Packages[1].Funcs, 1)
	assert.Len(t, in.WithTagged("slow").Packages[1].Files, 1, "a tag no file carries adds nothing")
}

// TestBehindTag reads a constraint for the tag, not negated.
func TestBehindTag(t *testing.T) {
	t.Parallel()

	assert.True(t, model.BehindTag([]string{"integration"}, "integration"))
	assert.True(t, model.BehindTag([]string{"(linux && integration)"}, "integration"))
	assert.False(t, model.BehindTag([]string{"!integration"}, "integration"))
	assert.False(t, model.BehindTag([]string{"integrations"}, "integration"))
}

// fileNames lists the files by name.
func fileNames(files model.FileList) []string {
	names := make([]string, 0, len(files))
	for _, file := range files {
		names = append(names, file.Name)
	}
	return names
}
