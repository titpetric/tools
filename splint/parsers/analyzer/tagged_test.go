package analyzer_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/titpetric/tools/splint/parsers/analyzer"
	"github.com/titpetric/tools/splint/pkg/splint"
)

// TestParseTagged covers the second load: a test file behind a build tag
// comes back typed, under DocumentRoot.Tagged, with the method it calls on a
// value resolved.
//
// The parse moves the process into the tree it reads, so this test runs
// alone.
func TestParseTagged(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":   "module example.com/tagged\n\ngo 1.22\n",
		"store.go": "package tagged\n\n// Store is a store.\ntype Store struct{}\n\n// Open opens it.\nfunc (s *Store) Open() {}\n",
		"store_integration_test.go": "//go:build integration\n\npackage tagged\n\nimport \"testing\"\n\n" +
			"func TestStore_Open(t *testing.T) {\n\ts := &Store{}\n\ts.Open()\n}\n",
	}
	for name, body := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
	}

	options := splint.Options{SourcePath: dir, Pattern: "./...", IncludeTests: true, IncludeTestFacts: true}
	root, err := analyzer.New(options).Parse(context.Background())
	require.NoError(t, err)

	require.Len(t, root.Tagged, 1)
	tagged := root.Tagged[0]
	assert.True(t, tagged.Package.TestPackage)
	assert.Equal(t, []string{"integration"}, tagged.Excluded["store_integration_test.go"])
	require.Len(t, tagged.Funcs, 1)
	assert.Equal(t, []string{"Store.Open"}, tagged.Funcs[0].Calls["example.com/tagged"])
}
