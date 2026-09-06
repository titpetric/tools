package imports_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/titpetric/tools/splint"
	"github.com/titpetric/tools/splint/linters/imports"
	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/simpleparser"
)

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

// files are the files a set of issues is about, in the order they were
// reported.
func files(issues []model.Issue) []string {
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		out = append(out, issue.Position.Path())
	}
	return out
}

// TestRulesOverTheUnformattedFixture states what each rule reports over the
// source that has never been formatted.
func TestRulesOverTheUnformattedFixture(t *testing.T) {
	byRule := fixture(t, inputDir, nil)

	// The order is the order the packages are walked in, which is by import
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
	// rewrite would damage. None of them is reported.
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
// is reached by is a guess. Reporting the import as unused would take out an
// import the file needs, and reporting yaml as unresolved would be an error
// against a file that builds. Neither is reported and the file is left alone.
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

	// broken.go is in the golden tree unchanged, because there was nothing to
	// write into it. It is still reported.
	assert.Equal(t, []string{"broken/broken.go"}, files(byRule[imports.RuleUnresolved]))
}

// TestPollutionLeavesLocalPackagesAlone is the rule the prompt draws the line
// at: a package of the tree reaching many consumers is a package structure,
// and only a dependency the tree took on is reported.
func TestPollutionLeavesLocalPackagesAlone(t *testing.T) {
	byRule := fixture(t, goldenDir, func(l *imports.Linter) {
		// One file of one package is enough to trip it, so every import that
		// could be reported is.
		l.Pollution = imports.Pollution{PerPackage: 1, FileShare: 0.01, Tests: true}
	})

	// The symbol is the dependency reported. Every package of this tree is
	// under example.com/imports and every one of them is reached from more
	// than one file, so any of them showing up here would be the rule judging
	// a package structure.
	for _, issue := range byRule[imports.RulePollution] {
		assert.NotContains(t, issue.Symbol, "example.com/imports",
			"a package of the tree was reported as pollution")
	}

	// Two dependencies this tree took on, and with the rule set to report one
	// file both of them are named.
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
