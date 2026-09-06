package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/titpetric/tools/splint/config"
	"github.com/titpetric/tools/splint/importfmt"
)

// TestLoadDefaults covers a tree with no file, which is read under the
// defaults and reports no error.
func TestLoadDefaults(t *testing.T) {
	cfg, err := config.Load(t.TempDir())
	require.NoError(t, err)

	assert.Equal(t, importfmt.DefaultOrder, cfg.Imports.Order)
	assert.Equal(t, 2, cfg.Imports.Pollution.PerPackage)
	assert.InDelta(t, 0.5, cfg.Imports.Pollution.FileShare, 0.0001)
	assert.True(t, cfg.Imports.Pollution.CountTests())

	opts := cfg.ImportOptions("example.com/app")
	assert.Equal(t, "example.com/app", opts.Project, "the project group defaults to the module")
	assert.True(t, opts.SetAlias)
}

// TestLoadFile covers a file that states everything.
func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(config.Path(dir), []byte(`imports:
  order: [std, general, project]
  company: [github.com/titpetric/]
  project: example.com/other
  set-alias: false
  aliases:
    assert: github.com/stretchr/testify/assert
  pollution:
    per-package: 4
    file-share: 0.25
    include-tests: false
stats:
  imports:
    fixed: 12
`), 0o644))

	cfg, err := config.Load(dir)
	require.NoError(t, err)

	assert.Equal(t, []string{"std", "general", "project"}, cfg.Imports.Order)
	assert.Equal(t, map[string]string{"assert": "github.com/stretchr/testify/assert"}, cfg.Imports.Aliases)
	assert.Equal(t, 4, cfg.Imports.Pollution.PerPackage)
	assert.False(t, cfg.Imports.Pollution.CountTests())
	assert.Equal(t, 12, cfg.Stats.Imports.Fixed)

	opts := cfg.ImportOptions("example.com/app")
	assert.Equal(t, "example.com/other", opts.Project, "a project the file names wins over the module")
	assert.Equal(t, []string{"github.com/titpetric/"}, opts.Company)
	assert.False(t, opts.SetAlias)
}

// TestAddFixedCreatesTheFile covers the first fix run against a tree that has
// no configuration.
func TestAddFixedCreatesTheFile(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, config.AddFixed(dir, 3))

	cfg, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, 3, cfg.Stats.Imports.Fixed)
}

// TestAddFixedAccumulates covers the counter being what the prompt asked for:
// how many times a file has been rewritten, across runs.
func TestAddFixedAccumulates(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, config.AddFixed(dir, 3))
	require.NoError(t, config.AddFixed(dir, 4))
	require.NoError(t, config.AddFixed(dir, 0))

	cfg, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, 7, cfg.Stats.Imports.Fixed)
}

// TestAddFixedKeepsComments is why the counter is written through the node
// tree rather than re-encoded from the struct.
func TestAddFixedKeepsComments(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(config.Path(dir), []byte(`# The house rule for this tree.
imports:
  # Everything under here is ours.
  company: [github.com/titpetric/]
`), 0o644))

	require.NoError(t, config.AddFixed(dir, 1))

	data, err := os.ReadFile(config.Path(dir))
	require.NoError(t, err)

	assert.Contains(t, string(data), "# The house rule for this tree.")
	assert.Contains(t, string(data), "# Everything under here is ours.")

	cfg, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, 1, cfg.Stats.Imports.Fixed)
	assert.Equal(t, []string{"github.com/titpetric/"}, cfg.Imports.Company)
}

// TestAddFixedLeavesNoTemporary covers the write: the file is replaced through
// a temporary, and the temporary is not left behind.
func TestAddFixedLeavesNoTemporary(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, config.AddFixed(dir, 1))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, config.Filename, filepath.Base(entries[0].Name()))
}
