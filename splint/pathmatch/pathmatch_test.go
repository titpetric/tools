package pathmatch_test

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/titpetric/tools/splint/pathmatch"
)

// TestMatch covers what the two stars are for: a single star that stops at a
// separator, and ** standing for any number of segments including none.
func TestMatch(t *testing.T) {
	tests := []struct {
		title   string
		pattern string
		name    string
		matched bool
	}{
		{title: "an exact name", pattern: "parser/parser.go", name: "parser/parser.go", matched: true},
		{title: "a star over one segment", pattern: "parser/*.go", name: "parser/parser.go", matched: true},
		{title: "a star does not cross a separator", pattern: "parser/*.go", name: "parser/sub/parser.go", matched: false},
		{title: "a star is not the whole path", pattern: "*.go", name: "parser/parser.go", matched: false},
		{title: "another directory", pattern: "parser/*.go", name: "runner/runner.go", matched: false},

		{title: "double star at a depth", pattern: "**/*_test.go", name: "linters/imports/imports_test.go", matched: true},
		{title: "double star standing for nothing", pattern: "**/*_test.go", name: "imports_test.go", matched: true},
		{title: "double star still reads the tail", pattern: "**/*_test.go", name: "linters/imports/imports.go", matched: false},
		{title: "double star alone", pattern: "**", name: "a/b/c.go", matched: true},
		{title: "double star under a prefix", pattern: "stdlib/**/*.go", name: "stdlib/crypto/hash/sha.go", matched: true},
		{title: "a prefix the name does not carry", pattern: "stdlib/**/*.go", name: "parser/parser.go", matched: false},

		{title: "a question mark", pattern: "v?/main.go", name: "v2/main.go", matched: true},
		{title: "a character class", pattern: "parser/[ps]*.go", name: "parser/stmt.go", matched: true},
		{title: "a character class that excludes", pattern: "parser/[ps]*.go", name: "parser/expr.go", matched: false},

		{title: "a pattern longer than the name", pattern: "a/b/c.go", name: "a/b", matched: false},
		{title: "a name longer than the pattern", pattern: "a/b", name: "a/b/c.go", matched: false},
	}

	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			matched, err := pathmatch.Match(test.pattern, test.name)
			require.NoError(t, err)
			assert.Equal(t, test.matched, matched)
		})
	}
}

// TestMatchBadPattern proves Match reports a malformed pattern for any
// name: the name here matches the readable half of the pattern, so an
// implementation that stopped matching before it reached the rest would return
// a verdict and drop the error.
func TestMatchBadPattern(t *testing.T) {
	for _, pattern := range []string{"[", "parser/[", "a[/b", "**/x[y"} {
		t.Run(pattern, func(t *testing.T) {
			matched, err := pathmatch.Match(pattern, "parser/parser.go")
			assert.ErrorIs(t, err, path.ErrBadPattern)
			assert.False(t, matched)
		})
	}
}

// TestValid is the check a loader runs, and it reads the whole pattern rather
// than the part a name happened to reach.
func TestValid(t *testing.T) {
	require.NoError(t, pathmatch.Valid("**/*_test.go"))
	require.NoError(t, pathmatch.Valid("parser/[ps]*.go"))
	assert.ErrorIs(t, pathmatch.Valid("parser/[ps*.go"), path.ErrBadPattern)
	assert.ErrorIs(t, pathmatch.Valid("[bad]]["), path.ErrBadPattern)
}
