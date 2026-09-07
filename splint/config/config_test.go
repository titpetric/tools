package config_test

import (
	"os"
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
`), 0o644))

	cfg, err := config.Load(dir)
	require.NoError(t, err)

	assert.Equal(t, []string{"std", "general", "project"}, cfg.Imports.Order)
	assert.Equal(t, map[string]string{"assert": "github.com/stretchr/testify/assert"}, cfg.Imports.Aliases)
	assert.Equal(t, 4, cfg.Imports.Pollution.PerPackage)
	assert.False(t, cfg.Imports.Pollution.CountTests())

	opts := cfg.ImportOptions("example.com/app")
	assert.Equal(t, "example.com/other", opts.Project, "a project the file names wins over the module")
	assert.Equal(t, []string{"github.com/titpetric/"}, opts.Company)
	assert.False(t, opts.SetAlias)
}
