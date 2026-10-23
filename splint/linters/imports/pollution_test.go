package imports_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/titpetric/tools/splint/linters/imports"
)

// ignoreTree writes a module whose one package reaches four dependencies from
// two files each, so the default thresholds report every one of them that the
// ignore list does not cover.
func ignoreTree(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	body := `package cli

import (
	_ "github.com/spf13/pflag"
	_ "golang.org/x/sync/errgroup"
	_ "golang.org/x/term"
	_ "golang.org/xx/thing"
)
`
	write := func(name, content string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	write("go.mod", "module example.com/ignore\n\ngo 1.27.0\n")
	write("cli/a.go", body)
	write("cli/b.go", body)

	return dir
}

// reported are the import paths the pollution rule names, read from the
// messages since the symbol is the owning module.
func reported(t *testing.T, tune func(*imports.Linter)) []string {
	t.Helper()

	var out []string
	for _, issue := range fixture(t, ignoreTree(t), tune)[imports.RulePollution] {
		for _, path := range []string{"github.com/spf13/pflag", "golang.org/x/sync/errgroup", "golang.org/x/term", "golang.org/xx/thing"} {
			if strings.HasPrefix(issue.Message, path+" ") {
				out = append(out, path)
			}
		}
	}
	return out
}

// TestPollutionIgnoresTheDefaultList covers the defaults: the golang.org/x
// modules and pflag stay out of the report, and golang.org/xx, which only
// shares the letters, does not.
func TestPollutionIgnoresTheDefaultList(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"golang.org/xx/thing"}, reported(t, nil))
}

// TestPollutionIgnoreCleared covers an empty list, which checks every
// dependency again.
func TestPollutionIgnoreCleared(t *testing.T) {
	t.Parallel()

	assert.ElementsMatch(t, []string{
		"github.com/spf13/pflag",
		"golang.org/x/sync/errgroup",
		"golang.org/x/term",
		"golang.org/xx/thing",
	}, reported(t, func(l *imports.Linter) { l.Pollution.Ignore = []string{} }))
}

// TestPollutionIgnoreReplaced covers a list of the tree's own, which takes
// the place of the default, so the rule reports golang.org/x/term again.
func TestPollutionIgnoreReplaced(t *testing.T) {
	t.Parallel()

	assert.ElementsMatch(t, []string{
		"github.com/spf13/pflag",
		"golang.org/x/term",
		"golang.org/xx/thing",
	}, reported(t, func(l *imports.Linter) { l.Pollution.Ignore = []string{"golang.org/x/sync"} }))
}

// TestPollutionIgnores covers the prefix match: a path segment boundary, a
// trailing slash read the same as none, and an empty entry covering nothing.
func TestPollutionIgnores(t *testing.T) {
	t.Parallel()

	p := imports.Pollution{Ignore: imports.DefaultIgnore}
	assert.True(t, p.Ignores("golang.org/x/sync/errgroup"))
	assert.True(t, p.Ignores("golang.org/x/term"))
	assert.True(t, p.Ignores("golang.org/x/crypto/ssh"))
	assert.True(t, p.Ignores("github.com/spf13/pflag"))
	assert.False(t, p.Ignores("golang.org/xx"))
	assert.False(t, p.Ignores("golang.org/xx/thing"))
	assert.False(t, p.Ignores("github.com/spf13/pflagx"))
	assert.False(t, p.Ignores("github.com/spf13/cobra"))

	slash := imports.Pollution{Ignore: []string{"golang.org/x/", ""}}
	assert.True(t, slash.Ignores("golang.org/x/term"))
	assert.False(t, slash.Ignores("golang.org/xx/thing"))
	assert.False(t, slash.Ignores("example.com/any"))

	assert.False(t, imports.Pollution{}.Ignores("golang.org/x/term"))
}
