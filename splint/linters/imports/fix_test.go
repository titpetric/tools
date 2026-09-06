package imports_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/titpetric/tools/splint"
	"github.com/titpetric/tools/splint/analyzer"
	"github.com/titpetric/tools/splint/config"
	"github.com/titpetric/tools/splint/fix"
	"github.com/titpetric/tools/splint/importfmt"
	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/simpleparser"
)

// The fixture: source written the way a file that has never been formatted
// writes it, and the same source as the formatter should leave it.
//
// testdata/output is what `atkins fix` writes, for reading a change by eye.
// This test writes its own copy into a temporary directory so a run of it
// neither depends on that directory nor leaves one behind.
const (
	inputDir  = "testdata/input"
	goldenDir = "testdata/golden"

	// cgoFile is the fixture that only one of the two parsers is always given.
	cgoFile = "cgo/cgo.go"
)

// TestFixGolden runs the fixer over the fixture and reports every file that
// does not come out the way testdata/golden says it should.
//
// Both parsers are run over the same input. A formatter that read one thing
// from the exact parser and another from the quick one would rewrite a file
// differently depending on a flag nobody passed.
func TestFixGolden(t *testing.T) {
	for _, parser := range []string{simpleparser.ParserName, analyzer.ParserName} {
		t.Run(parser, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "tree")
			copyTree(t, inputDir, dir)

			plan := planFor(t, parser, dir)

			changed, err := fix.Apply(plan)
			require.NoError(t, err)

			// The ast parser is given a cgo file only when the toolchain
			// compiles one. Under CGO_ENABLED=0 a file importing "C" is not
			// part of the package at all, so the parser never sees it and the
			// fixer has nothing to write. The quick parser reads the text and
			// always sees it.
			wanted := []string{
				"blackbox/blackbox_test.go",
				cgoFile,
				"client/client_test.go",
				"comments.go",
				"dangle/dangle.go",
				"dangle/merged.go",
				"main.go",
				"odd/empty.go",
				"odd/indented.go",
				"odd/oneline.go",
				"service1/handler/handler.go",
				"service2/storage/store.go",
				"unused.go",
			}
			sawCgo := slices.Contains(changed, cgoFile)
			if !sawCgo {
				wanted = slices.DeleteFunc(slices.Clone(wanted), func(name string) bool { return name == cgoFile })
			}
			assert.Equal(t, wanted, changed)

			// cgo/cgo.go is in that list because the block beside its
			// import of "C" is formatted like any other. What must not move
			// is the "C" declaration itself, which testdata/golden holds
			// exactly where the preamble above it put it.

			assert.Equal(t, []string{"broken/broken.go"}, plan.Skipped,
				"a file reaching a name nothing can place is left alone rather than written without the import it needs")

			compareTree(t, goldenDir, dir, sawCgo)
		})
	}
}

// TestFixIsIdempotent is the property a formatter has to have: running it over
// a tree it has already written changes nothing.
func TestFixIsIdempotent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tree")
	copyTree(t, goldenDir, dir)

	plan := planFor(t, simpleparser.ParserName, dir)

	changed, err := fix.Apply(plan)
	require.NoError(t, err)
	assert.Empty(t, changed, "the formatter rewrote a tree it had already written")
}

// planFor parses a tree and works out what the fixer would write.
func planFor(t *testing.T, parser, dir string) *fix.Plan {
	t.Helper()

	options := splint.Options{
		SourcePath:     dir,
		Pattern:        "./...",
		IncludeTests:   true,
		IncludeImports: true,
	}

	var (
		root *model.DocumentRoot
		err  error
	)
	switch parser {
	case analyzer.ParserName:
		root, err = analyzer.New(options).Parse(context.Background())
	default:
		root, err = simpleparser.New(options).Parse(context.Background())
	}
	require.NoError(t, err)

	settings := config.Default()
	return fix.Build(root, settings.ImportOptions(importfmt.Project(root)), settings.Imports.Aliases)
}

// compareTree reports every file of the tree that is not what the golden tree
// says it should be, and every file that is in one tree and not the other.
//
// A parse that never saw the cgo file leaves it as the input wrote it, which
// is the right answer for a parser the toolchain did not give it to.
func compareTree(t *testing.T, golden, got string, sawCgo bool) {
	t.Helper()

	want, have := names(t, golden), names(t, got)
	assert.Equal(t, want, have, "the formatter wrote a different set of files than the golden tree holds")

	for _, name := range want {
		expected := read(t, filepath.Join(golden, name))
		if name == cgoFile && !sawCgo {
			expected = read(t, filepath.Join(inputDir, name))
		}
		actual := read(t, filepath.Join(got, name))
		if expected == actual {
			continue
		}
		t.Errorf("%s is not formatted the way testdata/golden says:\n--- want\n%s\n--- got\n%s", name, expected, actual)
	}
}

// names are the files of a tree, relative to it and in order.
func names(t *testing.T, dir string) []string {
	t.Helper()

	var out []string
	require.NoError(t, filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)

		// A fix run against the tree writes its count into the tree. It is
		// not source and the golden tree does not hold one.
		if relative == config.Filename {
			return nil
		}
		out = append(out, relative)
		return nil
	}))

	sort.Strings(out)
	return out
}

// copyTree writes a copy of a tree, which is what gives a test a tree it may
// rewrite.
func copyTree(t *testing.T, from, to string) {
	t.Helper()

	require.NoError(t, filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relative, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, relative)

		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	}))
}

// read returns a file, with the trailing newline left on: a formatter that
// dropped one would be writing a file no other tool writes.
func read(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}
