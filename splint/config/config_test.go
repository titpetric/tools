package config_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/titpetric/tools/splint/config"
	"github.com/titpetric/tools/splint/importfmt"
	"github.com/titpetric/tools/splint/linters/filecheck"
)

// TestLoadDefaults covers a tree with no file, which gets the defaults and
// reports no error.
func TestLoadDefaults(t *testing.T) {
	cfg, err := config.Load(t.TempDir())
	require.NoError(t, err)

	assert.Equal(t, importfmt.DefaultOrder, cfg.Imports.Order)
	assert.Equal(t, 2, cfg.Imports.Pollution.PerPackage)
	assert.InDelta(t, 0.5, cfg.Imports.Pollution.FileShare, 0.0001)
	assert.True(t, cfg.Imports.Pollution.CountTests())

	assert.Equal(t, filecheck.DefaultMaxLines, cfg.Filecheck.MaxLines)
	assert.Empty(t, cfg.Filecheck.Allow, "a tree states its allowances or has none")

	opts := cfg.ImportOptions("example.com/app")
	assert.Equal(t, "example.com/app", opts.Project, "the project group defaults to the module")
	assert.True(t, opts.SetAlias)
}

// TestLoadFilecheckUnstated is the case the section must not break: a file
// that speaks about the imports and nothing else gets the same ceiling as a
// tree with no file at all.
func TestLoadFilecheckUnstated(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(config.Path(dir), []byte(`imports:
  company: [github.com/titpetric/]
`), 0o644))

	cfg, err := config.Load(dir)
	require.NoError(t, err)

	assert.Equal(t, filecheck.DefaultMaxLines, cfg.Filecheck.MaxLines)
	assert.Empty(t, cfg.FilecheckAllowances())
}

// TestLoadFilecheck reads the section the consumer requested, and checks it
// resolves the way the field docs say: the stated default for a file no entry
// covers, the entry's own ceiling for one it does, and a zero kept as a zero.
func TestLoadFilecheck(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(config.Path(dir), []byte(`filecheck:
  max-lines: 500
  allow:
    - path: parser/*.go
      max-lines: 1500
    - path: "**/*_test.go"
      max-lines: 900
    - path: internal/generated/*.go
      max-lines: 0
`), 0o644))

	cfg, err := config.Load(dir)
	require.NoError(t, err)

	assert.Equal(t, 500, cfg.Filecheck.MaxLines)
	require.Len(t, cfg.Filecheck.Allow, 3)
	assert.Equal(t, "parser/*.go", cfg.Filecheck.Allow[0].Path)
	assert.Equal(t, 1500, cfg.Filecheck.Allow[0].MaxLines)

	allow := cfg.FilecheckAllowances()
	require.Len(t, allow, 3)
	assert.Equal(t, 1500, allow.MaxLinesFor("parser/parser.go", cfg.Filecheck.MaxLines))
	assert.Equal(t, 900, allow.MaxLinesFor("flatstack/engine/vm_test.go", cfg.Filecheck.MaxLines))
	assert.Equal(t, 0, allow.MaxLinesFor("internal/generated/api.go", cfg.Filecheck.MaxLines))
	assert.Equal(t, 500, allow.MaxLinesFor("runner/runner.go", cfg.Filecheck.MaxLines),
		"a file no entry covers runs under the stated default")
}

// TestLoadFilecheckOffForTheTree covers the zero at the top of the section,
// which turns the rule off for every file no entry names.
func TestLoadFilecheckOffForTheTree(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(config.Path(dir), []byte(`filecheck:
  max-lines: 0
`), 0o644))

	cfg, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, 0, cfg.Filecheck.MaxLines)
}

// TestLoadFilecheckRefusesAMalformedEntry proves Load reports a typo at its
// source. A pattern nothing can read would otherwise cover no file for the
// life of the file, and the authors of the tree would think they had stated a
// rule.
func TestLoadFilecheckRefusesAMalformedEntry(t *testing.T) {
	tests := []struct {
		title string
		body  string
		want  string
	}{
		{
			title: "a pattern path.Match cannot read",
			body:  "filecheck:\n  allow:\n    - path: \"parser/[ps*.go\"\n      max-lines: 1500\n",
			want:  "is not a path pattern",
		},
		{
			title: "an entry naming no path",
			body:  "filecheck:\n  allow:\n    - max-lines: 1500\n",
			want:  "names no path",
		},
		{
			title: "a negative ceiling on an entry",
			body:  "filecheck:\n  allow:\n    - path: parser/*.go\n      max-lines: -1\n",
			want:  "not a number of lines",
		},
		{
			title: "a negative default",
			body:  "filecheck:\n  max-lines: -1\n",
			want:  "not a number of lines",
		},
		{
			title: "a ceiling that is not a number",
			body:  "filecheck:\n  max-lines: several\n",
			want:  "cannot unmarshal",
		},
	}

	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(config.Path(dir), []byte(test.body), 0o644))

			cfg, err := config.Load(dir)
			require.Error(t, err, "the file states a rule nothing can apply")
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), test.want)
			assert.Contains(t, err.Error(), config.Filename, "the error names the file it was read from")
		})
	}
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

// TestIntegration covers the section that turns the integration rule on, and
// a pattern under allow that no match can read.
func TestIntegration(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(config.Path(dir), []byte(`integration:
  enabled: true
  storage:
    - example.com/app/storage
  allow:
    - storage
    - runner/**
`), 0o644))

	cfg, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, config.Integration{
		Enabled: true,
		Storage: []string{"example.com/app/storage"},
		Allow:   []string{"storage", "runner/**"},
	}, cfg.Integration)

	require.NoError(t, os.WriteFile(config.Path(dir), []byte("integration:\n  allow: [\"[\"]\n"), 0o644))
	_, err = config.Load(dir)
	assert.ErrorContains(t, err, "integration.allow[0]")
}
