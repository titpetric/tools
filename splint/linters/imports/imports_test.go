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

	"github.com/titpetric/tools/splint/commands/fix"
	"github.com/titpetric/tools/splint/config"
	"github.com/titpetric/tools/splint/importfmt"
	"github.com/titpetric/tools/splint/linters/imports"
	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/parsers/analyzer"
	"github.com/titpetric/tools/splint/parsers/simpleparser"
	"github.com/titpetric/tools/splint/pkg/splint"
)

// lint runs the linter over one package's import set.
func lint(t *testing.T, set model.StringSet) []model.Issue {
	t.Helper()

	root := &model.DocumentRoot{Packages: model.DefinitionList{{
		Package: model.Package{Package: "x", Path: "./frontend/view"},
		Imports: set,
	}}}

	report, err := imports.New().Lint(context.Background(), root)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	return model.Issues(report)
}

// TestLinterFindsANameMeaningTwoThings covers what the compiler cannot: two
// files of one package reaching different modules under the same short name.
func TestLinterFindsANameMeaningTwoThings(t *testing.T) {
	issues := lint(t, model.StringSet{
		"a.go": {`"example.com/one/model"`},
		"b.go": {`"example.com/two/model"`},
	})

	if len(issues) != 1 {
		t.Fatalf("reported %d issues, want 1", len(issues))
	}
	if !strings.Contains(issues[0].Message, "model") {
		t.Errorf("message = %q, want the colliding name in it", issues[0].Message)
	}
	if issues[0].Rule != imports.RuleCollision {
		t.Errorf("rule = %q", issues[0].Rule)
	}
	// A collision belongs to the package and to neither of the two files.
	if issues[0].Position.Ref() != "frontend/view" {
		t.Errorf("position = %q, want the package", issues[0].Position.Ref())
	}
}

func TestLinterPassesAgreeingImports(t *testing.T) {
	issues := lint(t, model.StringSet{
		"a.go": {`"example.com/one/model"`, `"fmt"`},
		"b.go": {`"example.com/one/model"`},
		"c.go": {`two "example.com/two/model"`},
	})

	if len(issues) != 0 {
		t.Errorf("reported %d issues on imports that agree: %q", len(issues), issues[0].Message)
	}
}

func TestLinterOnTheRootPackage(t *testing.T) {
	root := &model.DocumentRoot{Packages: model.DefinitionList{{
		Package: model.Package{Package: "x", Path: "."},
		Imports: model.StringSet{
			"a.go": {`"example.com/one/model"`},
			"b.go": {`"example.com/two/model"`},
		},
	}}}

	report, err := imports.New().Lint(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for issue := range report.All() {
		if issue.Position.Ref() != "." {
			t.Errorf("position = %q, want the module root", issue.Position.Ref())
		}
	}
}

// The fixture: source written the way a file nobody ever formatted
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
// The test runs both parsers over the same input. A formatter that read one thing
// from the exact parser and another from the quick one would rewrite a file
// differently depending on a flag nobody passed.
func TestFixGolden(t *testing.T) {
	for _, parser := range []string{simpleparser.ParserName, analyzer.ParserName} {
		t.Run(parser, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "tree")
			copyTree(t, inputDir, dir)

			plan := planFor(t, parser, dir)

			changed, _, err := fix.Apply(plan)
			require.NoError(t, err)

			// The ast parser receives a cgo file only when the toolchain
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
			// import of "C" takes the same formatting as any other. What must not move
			// is the "C" declaration itself, which testdata/golden holds
			// exactly where the preamble above it put it.

			assert.Equal(t, []fix.Skip{{
				Name: "broken/broken.go",
				Unresolved: []importfmt.Unresolved{
					{Name: "nope", Symbols: []string{"Value"}},
				},
			}}, plan.Skipped,
				"a file reaching a name nothing can place is left alone, and the skip says which name and which use")

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

	changed, formatted, err := fix.Apply(plan)
	require.NoError(t, err)
	assert.Empty(t, changed, "the formatter rewrote a tree it had already written")
	assert.Empty(t, formatted, "the formatter reformatted a tree it had already written")
}

// planFor parses a tree and works out what the fixer would write.
func planFor(t *testing.T, parser, dir string) *fix.Plan {
	t.Helper()

	// The fixture is a copy of a module the toolchain loads, and the load must
	// not write into it: a go that adds a go.sum or an indirect requirement to
	// the copy would rewrite files the golden tree does not hold. The module
	// mode an environment exports is not part of what this test states, so pin
	// the one that reads the module and writes nothing.
	t.Setenv("GOFLAGS", "-mod=readonly")

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
// says, and every file that is in one tree and not the other.
//
// A parse that never saw the cgo file leaves it as the input wrote it, which
// is the right answer for a parser the toolchain kept that file from.
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

		// A fix run against the tree writes its count into the tree. That file is
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

// copyTree writes a copy of a tree, so a test has a tree it may
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

// fixture lints a tree and returns the issues, keyed by the rule they are
// under and the file they are in.
func fixture(t *testing.T, dir string, tune func(*imports.Linter)) map[string][]model.Issue {
	t.Helper()

	options := splint.Options{
		SourcePath:     dir,
		Pattern:        "./...",
		IncludeTests:   true,
		IncludeImports: true,
	}

	root, err := simpleparser.New(options).Parse(context.Background())
	require.NoError(t, err)

	linter := imports.New()
	if tune != nil {
		tune(linter)
	}

	report, err := linter.Lint(context.Background(), root)
	require.NoError(t, err)

	byRule := map[string][]model.Issue{}
	for issue := range report.All() {
		byRule[issue.Rule] = append(byRule[issue.Rule], issue)
	}

	return byRule
}

// files are the files a set of issues is about, in the order of the
// report.
func files(issues []model.Issue) []string {
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		out = append(out, issue.Position.Path())
	}
	return out
}

// TestRulesOverTheUnformattedFixture states what each rule reports over the
// source nobody has formatted yet.
func TestRulesOverTheUnformattedFixture(t *testing.T) {
	byRule := fixture(t, inputDir, nil)

	// The order is the walk order of the packages, which is by import
	// path: the root package first, then the ones under it.
	assert.Equal(t, []string{
		"comments.go",
		"main.go",
		"unused.go",
		"blackbox/blackbox_test.go",
		"cgo/cgo.go",
		"client/client_test.go",
		"dangle/dangle.go",
		"dangle/merged.go",
		"odd/empty.go",
		"odd/indented.go",
		"odd/oneline.go",
		"service1/handler/handler.go",
		"service2/storage/store.go",
	}, files(byRule[imports.RuleFormat]))

	// generated.go, goyaml.go and whitebox/whitebox_test.go are each a file a
	// rewrite would damage. The linter reports none of them.
	for _, issue := range byRule[imports.RuleFormat] {
		assert.NotContains(t, []string{"generated.go", "goyaml.go", "whitebox/whitebox_test.go"},
			issue.Position.Path(), "a file a rewrite would damage was reported as fixable")
	}

	require.Len(t, byRule[imports.RuleUnused], 1)
	unused := byRule[imports.RuleUnused][0]
	assert.Equal(t, "unused.go", unused.Position.Path())
	assert.Equal(t, "sort", unused.Symbol)
	assert.Equal(t, model.SeverityError, unused.Severity)

	require.Len(t, byRule[imports.RuleUnresolved], 1)
	unresolved := byRule[imports.RuleUnresolved][0]
	assert.Equal(t, "broken/broken.go", unresolved.Position.Path())
	assert.Equal(t, "nope", unresolved.Symbol)
	assert.Equal(t, model.SeverityError, unresolved.Severity)
	assert.Contains(t, unresolved.Message, "nope.Value")
}

// TestAPathThatDoesNotSpellItsPackage is the case that breaks a formatter
// reading the last segment of an import path as the package name.
//
// goyaml.go imports github.com/goccy/go-yaml and writes yaml. The last segment
// of that path is go-yaml, which is not a Go identifier, so the name the import
// goes by is a guess. Reporting the import as unused would take out an
// import the file needs, and reporting yaml as unresolved would be an error
// against a file that builds. The linter reports neither and leaves the file alone.
func TestAPathThatDoesNotSpellItsPackage(t *testing.T) {
	for _, dir := range []string{inputDir, goldenDir} {
		byRule := fixture(t, dir, nil)

		for _, rule := range []string{imports.RuleFormat, imports.RuleUnused, imports.RuleUnresolved} {
			for _, issue := range byRule[rule] {
				assert.NotEqual(t, "goyaml.go", issue.Position.Path(),
					"%s reported %s on an import whose name the path does not spell", dir, rule)
			}
		}
	}
}

// TestAnIndirectRequirementIsNotASecondImport covers the module a go.mod
// requires whose last path segment is the name a file writes for a different
// package.
//
// goyaml.go imports github.com/goccy/go-yaml and writes yaml, while the module
// graph of a real project also requires gopkg.in/yaml.v3: reading that
// requirement as the import the name needs would add a second yaml to a file
// that builds.
func TestAnIndirectRequirementIsNotASecondImport(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tree")
	copyTree(t, inputDir, dir)

	mod := filepath.Join(dir, "go.mod")
	require.NoError(t, os.WriteFile(mod, []byte(read(t, mod)+"\nrequire gopkg.in/yaml.v3 v3.0.1 // indirect\n"), 0o644))

	byRule := fixture(t, dir, nil)

	for _, rule := range []string{imports.RuleFormat, imports.RuleUnused, imports.RuleUnresolved} {
		for _, issue := range byRule[rule] {
			assert.NotEqual(t, "goyaml.go", issue.Position.Path(),
				"%s reported on a file whose import answers to a name a required module also ends in", rule)
		}
	}
}

// TestFixableIssuesSayWhatClearsThem covers the attribute a report carries so
// a reader knows a finding is one command away.
func TestFixableIssuesSayWhatClearsThem(t *testing.T) {
	byRule := fixture(t, inputDir, nil)

	for _, issue := range byRule[imports.RuleFormat] {
		fix, ok := issue.Attr(imports.FixAttr)
		assert.True(t, ok, "%s carries no fix attribute", issue.Position.Path())
		assert.Equal(t, imports.FixCommand, fix)
	}

	// A file nothing can place is not fixable: writing the block would leave
	// out an import the file needs.
	for _, issue := range byRule[imports.RuleUnresolved] {
		_, ok := issue.Attr(imports.FixAttr)
		assert.False(t, ok, "an unresolved name is not something a rewrite clears")
	}
}

// TestTheFormattedFixturePasses is the other half of the golden test, read
// through the linter: the tree the formatter writes reports nothing.
func TestTheFormattedFixturePasses(t *testing.T) {
	byRule := fixture(t, goldenDir, nil)

	assert.Empty(t, files(byRule[imports.RuleFormat]))
	assert.Empty(t, files(byRule[imports.RuleUnused]))

	// broken.go sits in the golden tree unchanged, because the fixer had nothing to
	// write into it. The linter still reports it.
	assert.Equal(t, []string{"broken/broken.go"}, files(byRule[imports.RuleUnresolved]))
}

// TestPollutionLeavesLocalPackagesAlone is the rule the prompt draws the line
// at: a package of the tree reaching many consumers is a package structure,
// and the linter reports only a dependency the tree took on.
func TestPollutionLeavesLocalPackagesAlone(t *testing.T) {
	byRule := fixture(t, goldenDir, func(l *imports.Linter) {
		// One file of one package is enough to trip it, so the linter reports
		// every import it can.
		l.Pollution = imports.Pollution{PerPackage: 1, FileShare: 0.01, Tests: true}
	})

	// The symbol is the dependency reported. Every package of this tree is
	// under example.com/imports and more than one file reaches
	// each of them, so any of them showing up here would be the rule judging
	// a package structure.
	for _, issue := range byRule[imports.RulePollution] {
		assert.NotContains(t, issue.Symbol, "example.com/imports",
			"a package of the tree was reported as pollution")
	}

	// Two dependencies this tree took on, and with the rule set to report one
	// file the report names both of them.
	var reported []string
	for _, issue := range byRule[imports.RulePollution] {
		reported = append(reported, issue.Symbol)
		assert.Equal(t, model.SeverityWarn, issue.Severity)
	}
	assert.ElementsMatch(t, []string{"github.com/stretchr/testify", "github.com/goccy/go-yaml"}, reported)
}

// TestPollutionCountsAPackageAndItsTestsTogether covers what the default
// reports: two files of one directory reaching one dependency, one of them a
// test file.
//
// A package and its test half are two definitions of one directory in the
// model, and the count is per directory. Counting them apart would let a
// dependency reach every test of a package without the rule saying so.
func TestPollutionCountsAPackageAndItsTestsTogether(t *testing.T) {
	byRule := fixture(t, goldenDir, nil)

	require.Len(t, byRule[imports.RulePollution], 1)
	assert.Contains(t, byRule[imports.RulePollution][0].Message, "2 of them in client alone")
}

// TestPollutionSkipsTestFilesWhenAsked covers the knob the count is noisy
// without: a package whose test files all reach one dependency.
func TestPollutionSkipsTestFilesWhenAsked(t *testing.T) {
	counted := fixture(t, goldenDir, func(l *imports.Linter) {
		l.Pollution = imports.Pollution{PerPackage: 2, Tests: true}
	})
	require.Len(t, counted[imports.RulePollution], 1,
		"client.go and client_test.go are two files of one package reaching testify")

	skipped := fixture(t, goldenDir, func(l *imports.Linter) {
		l.Pollution = imports.Pollution{PerPackage: 2, Tests: false}
	})
	assert.Empty(t, skipped[imports.RulePollution],
		"without the test file only client.go reaches it, which is one place")
}
