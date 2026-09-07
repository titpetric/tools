package config_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/titpetric/tools/splint/config"
)

// TestAddFixedCreatesTheFile covers the first fix run on a machine that has
// no counter yet.
func TestAddFixedCreatesTheFile(t *testing.T) {
	userConfig(t)

	require.NoError(t, config.AddFixed(3))

	stats, err := config.LoadStats()
	require.NoError(t, err)
	assert.Equal(t, 3, stats.Imports.Fixed)
}

// TestAddFixedAccumulates covers the counter being what it says: how many
// times a file has been rewritten, across runs and across trees.
func TestAddFixedAccumulates(t *testing.T) {
	userConfig(t)

	require.NoError(t, config.AddFixed(3))
	require.NoError(t, config.AddFixed(4))
	require.NoError(t, config.AddFixed(0))

	stats, err := config.LoadStats()
	require.NoError(t, err)
	assert.Equal(t, 7, stats.Imports.Fixed)
}

// TestAddFixedKeepsComments is why the counter is written through the node
// tree rather than re-encoded from the struct.
func TestAddFixedKeepsComments(t *testing.T) {
	userConfig(t)

	path, err := config.StatsPath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(`# What splint has counted on this machine.
imports:
  # Files whose import block was rewritten.
  fixed: 5
`), 0o644))

	require.NoError(t, config.AddFixed(1))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "# What splint has counted on this machine.")
	assert.Contains(t, string(data), "# Files whose import block was rewritten.")

	stats, err := config.LoadStats()
	require.NoError(t, err)
	assert.Equal(t, 6, stats.Imports.Fixed)
}

// TestAddFixedIsNotWrittenBesideTheTree is the change this test file exists
// for: a run that rewrote a file leaves the tree it rewrote alone.
func TestAddFixedIsNotWrittenBesideTheTree(t *testing.T) {
	userConfig(t)

	tree := t.TempDir()
	require.NoError(t, config.AddFixed(2))

	entries, err := os.ReadDir(tree)
	require.NoError(t, err)
	assert.Empty(t, entries, "the counter was written into the tree")
}

// TestLoadStatsWithoutAFile covers a machine that has never run the fixer.
func TestLoadStatsWithoutAFile(t *testing.T) {
	userConfig(t)

	stats, err := config.LoadStats()
	require.NoError(t, err)
	assert.Equal(t, 0, stats.Imports.Fixed)
}

// userConfig points the counter at a directory of the test's own, so a run of
// the suite neither reads nor writes the counter of whoever is running it.
func userConfig(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	return dir
}
