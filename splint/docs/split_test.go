package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/titpetric/tools/splint/model"
)

func TestSplitFilename(t *testing.T) {
	tests := []struct {
		name       string
		importPath string
		strip      []string
		expected   string
	}{
		{
			name:       "with strip prefix",
			importPath: "github.com/titpetric/atkins/runner/view",
			strip:      []string{"github.com/titpetric"},
			expected:   "atkins_runner_view.md",
		},
		{
			name:       "strip exact root",
			importPath: "github.com/titpetric/atkins",
			strip:      []string{"github.com/titpetric"},
			expected:   "atkins.md",
		},
		{
			name:       "without strip, exclude first element",
			importPath: "github.com/titpetric/atkins/runner",
			expected:   "titpetric_atkins_runner.md",
		},
		{
			name:       "without strip, single element",
			importPath: "mypackage",
			expected:   "mypackage.md",
		},
		{
			name:       "strip not matching",
			importPath: "github.com/other/package",
			strip:      []string{"github.com/titpetric"},
			expected:   "github.com_other_package.md",
		},
		{
			name:       "strip with trailing slash",
			importPath: "github.com/titpetric/atkins/runner",
			strip:      []string{"github.com/titpetric/"},
			expected:   "atkins_runner.md",
		},
		{
			name:       "second prefix matches",
			importPath: "github.com/titpetric/atkins/runner",
			strip:      []string{"example.com/", "github.com/titpetric/"},
			expected:   "atkins_runner.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, splitFilename(tt.importPath, tt.strip))
		})
	}
}

func TestGroupDefinitionsByPackage(t *testing.T) {
	defs := model.DefinitionList{
		{Package: model.Package{ImportPath: "github.com/user/pkg1"}},
		{Package: model.Package{ImportPath: "github.com/user/pkg1"}},
		{Package: model.Package{ImportPath: "github.com/user/pkg2"}},
		{Package: model.Package{ImportPath: "github.com/user/pkg3", TestPackage: true}},
	}

	groups := groupDefinitionsByPackage(defs)

	require.Len(t, groups, 2, "should have 2 groups (test package excluded)")
	require.Len(t, groups["github.com/user/pkg1"], 2, "pkg1 should have 2 definitions")
	require.Len(t, groups["github.com/user/pkg2"], 1, "pkg2 should have 1 definition")
	require.NotContains(t, groups, "github.com/user/pkg3", "test package should be excluded")
}

// splitDefs is a small tree of packages for the split tests.
func splitDefs(importPaths ...string) model.DefinitionList {
	var defs model.DefinitionList
	for _, importPath := range importPaths {
		defs = append(defs, &model.Definition{
			Package: model.Package{
				ImportPath: importPath,
				Path:       "./" + filepath.Base(importPath),
				Package:    filepath.Base(importPath),
			},
			Doc: "Package " + filepath.Base(importPath),
		})
	}
	return defs
}

func TestRenderSplit(t *testing.T) {
	tmpDir := t.TempDir()

	opts := Options{
		Split:       true,
		OutDir:      tmpDir,
		StripPrefix: []string{"github.com/test"},
	}

	err := renderSplit(splitDefs("github.com/test/pkg1", "github.com/test/pkg2"), opts)
	require.NoError(t, err)

	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)

	fileNames := make(map[string]bool)
	for _, f := range files {
		fileNames[f.Name()] = true
	}

	require.True(t, fileNames["README.md"], "README.md should be created")
	require.True(t, fileNames["pkg1.md"], "pkg1.md should be created")
	require.True(t, fileNames["pkg2.md"], "pkg2.md should be created")

	content, err := os.ReadFile(filepath.Join(tmpDir, "README.md"))
	require.NoError(t, err)

	readmeText := string(content)
	require.Contains(t, readmeText, "# API Documentation")
	require.Contains(t, readmeText, "## Table of Contents")
	require.Contains(t, readmeText, "[github.com/test/pkg1](./pkg1.md)")
	require.Contains(t, readmeText, "[github.com/test/pkg2](./pkg2.md)")
}

func TestRenderSplitCreatesMissingDir(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "nested", "output")

	opts := Options{Split: true, OutDir: outDir, StripPrefix: []string{"github.com/test"}}

	err := renderSplit(splitDefs("github.com/test/pkg1"), opts)
	require.NoError(t, err)

	info, err := os.Stat(outDir)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestRenderSplitExcludesTestPackages(t *testing.T) {
	tmpDir := t.TempDir()

	defs := splitDefs("github.com/test/pkg1")
	defs = append(defs, &model.Definition{
		Package: model.Package{
			ImportPath:  "github.com/test/pkg1_test",
			Path:        "./pkg1",
			Package:     "pkg1_test",
			TestPackage: true,
		},
	})

	opts := Options{Split: true, OutDir: tmpDir, StripPrefix: []string{"github.com/test"}}

	err := renderSplit(defs, opts)
	require.NoError(t, err)

	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)

	fileNames := make(map[string]bool)
	for _, f := range files {
		fileNames[f.Name()] = true
	}

	require.True(t, fileNames["pkg1.md"], "pkg1.md should exist")
	require.False(t, fileNames["pkg1_test.md"], "test package should not create a file")
}

func TestRenderSplitMultiplePackagesOrdering(t *testing.T) {
	tmpDir := t.TempDir()

	opts := Options{Split: true, OutDir: tmpDir, StripPrefix: []string{"github.com/test"}}

	err := renderSplit(splitDefs("github.com/test/zeta", "github.com/test/alpha", "github.com/test/beta"), opts)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(tmpDir, "README.md"))
	require.NoError(t, err)

	readmeText := string(content)
	alphaIdx := strings.Index(readmeText, "github.com/test/alpha")
	betaIdx := strings.Index(readmeText, "github.com/test/beta")
	zetaIdx := strings.Index(readmeText, "github.com/test/zeta")

	require.True(t, alphaIdx < betaIdx, "alpha should come before beta")
	require.True(t, betaIdx < zetaIdx, "beta should come before zeta")
}

func TestRenderSplitWithComplexImportPath(t *testing.T) {
	tmpDir := t.TempDir()

	defs := model.DefinitionList{{
		Package: model.Package{
			ImportPath: "github.com/titpetric/atkins/runner/view",
			Path:       "./runner/view",
			Package:    "view",
		},
	}}

	opts := Options{Split: true, OutDir: tmpDir, StripPrefix: []string{"github.com/titpetric"}}

	err := renderSplit(defs, opts)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(tmpDir, "atkins_runner_view.md"))
	require.NoError(t, err, "atkins_runner_view.md should exist")
}

func TestPackageMarkdown(t *testing.T) {
	def := &model.Definition{
		Package: model.Package{
			ImportPath: "github.com/test/pkg1",
			Path:       "./pkg1",
			Package:    "pkg1",
		},
		Doc: "This is a test package",
		Types: model.DeclarationList{
			{Name: "MyType", Source: "type MyType struct { Field string }"},
		},
		Funcs: model.DeclarationList{
			{
				Name:      "DoSomething",
				Signature: "DoSomething(x int) string",
				Doc:       "DoSomething does something important",
				Source:    "func DoSomething(x int) string { return \"\" }",
			},
		},
	}

	content := packageMarkdown(def, nil)

	require.Contains(t, content, "# Package ./pkg1")
	require.Contains(t, content, "import (\n\t\"github.com/test/pkg1\"\n)")
	require.Contains(t, content, "This is a test package")
	require.Contains(t, content, "## Types")
	require.Contains(t, content, "type MyType struct { Field string }")
	require.Contains(t, content, "## Function symbols")
	require.Contains(t, content, "DoSomething does something important")
	require.Contains(t, content, "func DoSomething(x int) string")
}
