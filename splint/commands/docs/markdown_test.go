package docs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/titpetric/tools/splint/model"
)

// The page cites the package name and every function heading as code.
func TestPackageMarkdownCitesSymbols(t *testing.T) {
	def := &model.Definition{
		Package: model.Package{ImportPath: "example.com/config", Path: "./config", Package: "config"},
		Doc:     "Package config reads the settings.",
		Funcs: model.DeclarationList{
			{Name: "Validate", Doc: "Validate checks the settings."},
			{Name: "Load"},
		},
	}

	content := packageMarkdown(def, nil)

	require.Contains(t, content, "Package `config` reads the settings.")
	require.Contains(t, content, "### `Validate`\n\n`Validate` checks the settings.")
	require.Contains(t, content, "### `Load`\n\n")
}

func TestCitePackage(t *testing.T) {
	tests := []struct{ doc, name, want string }{
		{"Package config reads it.", "config", "Package `config` reads it."},
		{"Package config", "config", "Package `config`"},
		{"Package configs reads it.", "config", "Package configs reads it."},
		{"Reads the settings.", "config", "Reads the settings."},
		{"Package config reads it.", "", "Package config reads it."},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, citePackage(tt.doc, tt.name), tt.doc)
	}
}

func TestCiteName(t *testing.T) {
	require.Equal(t, "`Load` reads it.", citeName("Load reads it.", "Load"))
	require.Equal(t, "Loader reads it.", citeName("Loader reads it.", "Load"))
	require.Equal(t, "Reads it.", citeName("Reads it.", "Load"))
}
