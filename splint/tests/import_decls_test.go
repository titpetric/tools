package tests_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/titpetric/tools/splint"
	"github.com/titpetric/tools/splint/analyzer"
	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/simpleparser"
)

// scattered writes its imports the way a file that has never been through a
// formatter writes them: two declarations, a comment above one spec and beside
// another, a blank line inside a group, an alias that repeats the path and one
// that does not.
const scattered = `package scattered

import (
	"fmt"

	// Postgres registers itself.
	_ "example.com/pq"
	json "encoding/json" // the alias repeats the path
	// A note belonging to no import at all.
)

import "strings"

// Encode writes v as JSON.
func Encode(v any) string {
	out, _ := json.Marshal(v)
	return fmt.Sprint(strings.TrimSpace(string(out)))
}
`

// TestImportDeclsAcrossParsers is the claim the fixer rests on: the two
// parsers read the same import declarations off the same file.
//
// A formatter that read one thing from the ast parser and another from the
// quick one would rewrite a file differently depending on a flag nobody
// passed, so the two are compared here rather than each being checked against
// what it was expected to say.
func TestImportDeclsAcrossParsers(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module example.com/scattered\n\ngo 1.27.0\n")
	write(t, filepath.Join(dir, "scattered.go"), scattered)

	options := splint.Options{SourcePath: dir, Pattern: ".", IncludeImports: true}

	ast, err := analyzer.New(options).Parse(context.Background())
	require.NoError(t, err)
	simple, err := simpleparser.New(options).Parse(context.Background())
	require.NoError(t, err)

	astFile := onlyFile(t, ast)
	simpleFile := onlyFile(t, simple)

	assert.Equal(t, astFile.ImportDecls, simpleFile.ImportDecls, "the two parsers read different import declarations")
	assert.Equal(t, astFile.Uses, simpleFile.Uses, "the two parsers read different uses")

	require.Len(t, astFile.ImportDecls, 2, "two import declarations, not one merged set")
	assert.True(t, astFile.ImportDecls[0].Block)
	assert.False(t, astFile.ImportDecls[1].Block)

	specs := astFile.ImportDecls.Specs()
	require.Len(t, specs, 4)

	assert.Equal(t, "fmt", specs[0].Path)
	assert.Equal(t, "", specs[0].Name)

	assert.Equal(t, "example.com/pq", specs[1].Path)
	assert.Equal(t, "_", specs[1].Name, "a blank import keeps its underscore")
	assert.Equal(t, "Postgres registers itself.", specs[1].Doc)
	assert.True(t, specs[1].Break, "the blank line above it is a group boundary")

	assert.Equal(t, "encoding/json", specs[2].Path)
	assert.Equal(t, "json", specs[2].Name, "an alias that repeats the path is still what the file wrote")
	assert.Equal(t, "the alias repeats the path", specs[2].Comment, "a comment is recorded with its markers trimmed, the way a doc comment is")

	assert.Equal(t, "strings", specs[3].Path)

	// The comment that belongs to no import is the declaration's, and both
	// parsers have to agree on it: a rewrite that dropped it on one parser and
	// kept it on the other would write two different files.
	assert.Equal(t, "A note belonging to no import at all.", astFile.ImportDecls[0].Tail)
	assert.Equal(t, "", astFile.ImportDecls[1].Tail)

	// The package clause is what tells a white box test file from a black box
	// one, which is what decides whether a file may import the package it is
	// beside.
	assert.Equal(t, "scattered", astFile.Package)

	// Uses over-collects on purpose, so the three package names are in it and
	// nothing that is not a selector is.
	assert.Equal(t, []string{"fmt", "json", "strings"}, astFile.Uses)
}

// TestImportDeclsAreNotFilledUnlessAsked keeps the parity harness honest: a
// parse that did not ask for the import declarations produces the document it
// always produced.
func TestImportDeclsAreNotFilledUnlessAsked(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module example.com/scattered\n\ngo 1.27.0\n")
	write(t, filepath.Join(dir, "scattered.go"), scattered)

	options := splint.Options{SourcePath: dir, Pattern: "."}

	for _, parser := range []splint.Parser{analyzer.New(options), simpleparser.New(options)} {
		doc, err := parser.Parse(context.Background())
		require.NoError(t, err)

		file := onlyFile(t, doc)
		assert.Empty(t, file.ImportDecls)
		assert.Empty(t, file.Uses)
	}
}

// onlyFile returns the one file of the one package a document holds.
func onlyFile(t *testing.T, doc *model.DocumentRoot) model.File {
	t.Helper()

	require.Len(t, doc.Packages, 1)
	require.Len(t, doc.Packages[0].Files, 1)
	return doc.Packages[0].Files[0]
}
