package filecheck_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/titpetric/tools/splint/linters/filecheck"
)

// TestAllowances_MaxLinesFor reads the resolution on its own: which entry
// applies, what a file no entry covers runs under, and that a zero stays a
// zero.
func TestAllowances_MaxLinesFor(t *testing.T) {
	allow := filecheck.Allowances{
		{Path: "parser/*.go", MaxLines: 1500},
		{Path: "**/*_test.go", MaxLines: 900},
		{Path: "internal/generated/*.go", MaxLines: 0},
	}

	tests := []struct {
		title string
		path  string
		want  int
	}{
		{title: "the first entry", path: "parser/parser.go", want: 1500},
		{title: "the second entry at a depth", path: "flatstack/engine/vm_test.go", want: 900},
		{title: "the second entry in the root", path: "main_test.go", want: 900},
		{title: "a zero is a ceiling and not an absence", path: "internal/generated/api.go", want: 0},
		{title: "a file no entry covers", path: "runner/runner.go", want: 400},
		{title: "a directory a star does not reach", path: "parser/php/parser.go", want: 400},
	}

	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			assert.Equal(t, test.want, allow.MaxLinesFor(test.path, 400))
		})
	}
}

// TestAllowances_MaxLinesFor_FirstMatchWins is the rule the field doc states.
// Both entries cover the file and the one written first is the one that
// applies, so an author orders the narrow entry above the broad one.
func TestAllowances_MaxLinesFor_FirstMatchWins(t *testing.T) {
	narrow := filecheck.Allowances{
		{Path: "parser/*_test.go", MaxLines: 2000},
		{Path: "**/*_test.go", MaxLines: 900},
	}
	assert.Equal(t, 2000, narrow.MaxLinesFor("parser/parser_test.go", 400))

	broad := filecheck.Allowances{
		{Path: "**/*_test.go", MaxLines: 900},
		{Path: "parser/*_test.go", MaxLines: 2000},
	}
	assert.Equal(t, 900, broad.MaxLinesFor("parser/parser_test.go", 400),
		"the broad entry was written first, so it is the one that applies")
}

// TestAllowances_MaxLinesFor_Empty covers the linter as the registry builds it,
// where there are no allowances at all.
func TestAllowances_MaxLinesFor_Empty(t *testing.T) {
	assert.Equal(t, 400, filecheck.Allowances(nil).MaxLinesFor("parser/parser.go", 400))
}

// TestAllowances_MaxLinesFor_BadPattern proves a pattern the loader would have
// refused covers nothing and keeps a run going. The loader reports it; a
// linter handed one has no reader to report it to.
func TestAllowances_MaxLinesFor_BadPattern(t *testing.T) {
	allow := filecheck.Allowances{{Path: "parser/[", MaxLines: 1500}}
	assert.Equal(t, 400, allow.MaxLinesFor("parser/parser.go", 400))
}
