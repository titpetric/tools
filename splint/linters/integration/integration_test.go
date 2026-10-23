package integration_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/titpetric/tools/splint/linters/integration"
	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/parsers/analyzer"
	"github.com/titpetric/tools/splint/parsers/simpleparser"
	"github.com/titpetric/tools/splint/pkg/splint"
)

// lint runs an enabled linter and returns its issues, checking on the way
// what every report owes the framework.
func lint(t *testing.T, linter *integration.Linter, root *model.DocumentRoot) (model.LintReport, []model.Issue) {
	t.Helper()

	report, err := linter.Lint(context.Background(), root)
	require.NoError(t, err)
	assert.Equal(t, integration.Name, report.Linter())

	issues := model.Issues(report)
	assert.Len(t, issues, report.Len())
	return report, issues
}

// enabled is the linter as a tree that opted in runs it.
func enabled() *integration.Linter {
	linter := integration.New()
	linter.Enabled = true
	return linter
}

// byRule is the symbols reported under each rule, sorted.
func byRule(issues []model.Issue) map[string][]string {
	out := map[string][]string{}
	for _, issue := range issues {
		out[issue.Rule] = append(out[issue.Rule], issue.Symbol)
	}
	for rule := range out {
		sort.Strings(out[rule])
	}
	return out
}

// TestLint parses the fixture under testdata with both parsers: the two
// have to agree on every finding.
//
// The ast parser moves the process into the tree it reads, so this test runs
// alone.
func TestLint(t *testing.T) {
	for _, name := range []string{simpleparser.ParserName, analyzer.ParserName} {
		t.Run(name, func(t *testing.T) {
			options := splint.Options{
				SourcePath:       "testdata",
				Pattern:          "./...",
				IncludeTests:     true,
				IncludeImports:   true,
				IncludeTestFacts: true,
			}
			var (
				root *model.DocumentRoot
				err  error
			)
			if name == analyzer.ParserName {
				root, err = analyzer.New(options).Parse(context.Background())
			} else {
				root, err = simpleparser.New(options).Parse(context.Background())
			}
			require.NoError(t, err)

			linter := enabled()
			linter.Allow = []string{"store"}
			report, issues := lint(t, linter, root)

			assert.Equal(t, map[string][]string{
				integration.RuleBoundary: {"TestExec", "TestHelper", "TestServer", "TestStore"},
				integration.RuleFilename: {"misnamed_test.go"},
				integration.RuleTag:      {"loose_integration_test.go"},
				integration.RuleOutside:  {"db_integration_test.go", "misnamed_test.go"},
			}, byRule(issues))

			for _, issue := range issues {
				if issue.Symbol != "TestHelper" {
					continue
				}
				assert.Equal(t, "api/api_test.go", issue.Position.File)
				assert.Contains(t, issue.Message, "calls openMemory (helpers_test.go), which opens sqlite")
				assert.Contains(t, issue.Message, "move it to api_integration_test.go with //go:build integration")
			}

			metric, ok := report.Metrics().Packages["./api"].(integration.Metric)
			require.True(t, ok, "metrics hold nothing for ./api")
			assert.Equal(t, integration.Metric{Tests: 6, Crossing: 4, Integration: 2}, metric)
			assert.NotEmpty(t, report.Statistics())
		})
	}
}

// TestLintDisabled is the default: a tree that never opted in reads clean.
func TestLintDisabled(t *testing.T) {
	t.Parallel()

	root := &model.DocumentRoot{Packages: model.DefinitionList{{
		Package:  model.Package{Package: "api_test", Path: "./api", TestPackage: true},
		Files:    model.FileList{{Name: "loose_integration_test.go", Test: true}},
		Excluded: model.StringSet{"misnamed_test.go": {"integration"}},
	}}}

	_, issues := lint(t, integration.New(), root)
	assert.Empty(t, issues)
}

// TestLintImportsAndStorage reads a document written by hand: a file whose
// only crossing is a driver import, a storage path the tree names, a
// negated tag, and a tag the tree renames.
func TestLintImportsAndStorage(t *testing.T) {
	t.Parallel()

	test := func(name, file string, refs model.StringSet) *model.Declaration {
		return &model.Declaration{Kind: model.FuncKind, Name: name, File: file, Line: 3, References: refs}
	}
	root := &model.DocumentRoot{Packages: model.DefinitionList{{
		Package: model.Package{Package: "app_test", Path: "./app", TestPackage: true},
		Files: model.FileList{
			{Name: "driver_test.go", Test: true},
			{Name: "repo_test.go", Test: true},
			{Name: "net_test.go", Test: true},
		},
		Imports: model.StringSet{
			"driver_test.go": {`_ "modernc.org/sqlite"`},
			"repo_test.go":   {`"example.com/app/repo"`, `"example.com/app/storage"`},
			"net_test.go":    {`"net"`},
		},
		Excluded: model.StringSet{
			"unit_test.go":  {"!slow"},
			"slow_test.go":  {"slow && linux"},
			"other_test.go": {"integration"},
		},
		Funcs: model.DeclarationList{
			test("TestDriver", "driver_test.go", nil),
			test("TestRepo", "repo_test.go", model.StringSet{"repo": {"Open"}, "storage": {"New"}}),
			test("TestIP", "net_test.go", model.StringSet{"net": {"ParseIP"}}),
			test("TestDial", "net_test.go", model.StringSet{"net": {"Dial"}}),
		},
	}}}

	linter := enabled()
	linter.Tag = "slow"
	linter.Storage = []string{"example.com/app/repo"}
	_, issues := lint(t, linter, root)

	assert.Equal(t, map[string][]string{
		integration.RuleBoundary: {"TestDial", "TestRepo", "driver_test.go"},
		integration.RuleFilename: {"slow_test.go"},
	}, byRule(issues))

	for _, issue := range issues {
		switch issue.Symbol {
		case "TestRepo":
			reason, _ := issue.Attr("reason")
			assert.Equal(t, "imports example.com/app/repo and calls repo.Open", reason)
		case "driver_test.go":
			assert.True(t, strings.HasPrefix(issue.Message, "driver_test.go imports modernc.org/sqlite;"), issue.Message)
		}
	}
}
