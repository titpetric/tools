package simpleparser_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/parsers/simpleparser"
	"github.com/titpetric/tools/splint/pkg/splint"
)

// TestReadTagged reads the test files a document names as kept out of the
// build into a test definition of their directory.
func TestReadTagged(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "store")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "store_integration_test.go"), []byte(
		"//go:build integration\n\npackage store_test\n\nimport \"testing\"\n\nfunc TestOpen(t *testing.T) {}\n"), 0o644))

	doc := &model.DocumentRoot{Root: root, Packages: model.DefinitionList{{
		Package:  model.Package{Package: "store", ImportPath: "example.com/store", Path: "./store"},
		Excluded: model.StringSet{"store_integration_test.go": {"integration"}},
	}}}
	require.NoError(t, simpleparser.ReadTagged(doc, splint.Options{}))

	require.Len(t, doc.Tagged, 1)
	tagged := doc.Tagged[0]
	assert.Equal(t, model.Package{ID: "example.com/store_test", Package: "store_test", ImportPath: "example.com/store_test", Path: "./store", TestPackage: true}, tagged.Package)
	assert.Equal(t, []string{"integration"}, tagged.Excluded["store_integration_test.go"])
	require.Len(t, tagged.Funcs, 1)
	assert.Equal(t, "TestOpen", tagged.Funcs[0].Name)
}
