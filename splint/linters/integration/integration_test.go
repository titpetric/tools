package integration_test

import (
	"context"
	"sort"
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

			// The files behind the tag come back as a test definition of
			// their own, for the linters that count tests.
			require.Len(t, root.Tagged, 1)
			assert.Equal(t, "./api", root.Tagged[0].Package.Path)
			assert.ElementsMatch(t, []string{"db_integration_test.go", "misnamed_test.go"}, root.Tagged[0].Excluded.Keys())
			assert.Len(t, root.Tagged[0].Funcs, 2)

			report, issues := lint(t, enabled(), root)

			assert.Equal(t, map[string][]string{
				integration.RuleBoundary: {"TestExec", "TestHelper", "TestServer", "TestStart", "TestStore"},
				integration.RuleFilename: {"misnamed_test.go"},
				integration.RuleTag:      {"loose_integration_test.go"},
			}, byRule(issues))

			for _, issue := range issues {
				switch issue.Symbol {
				case "TestHelper":
					assert.Equal(t, "api/api_test.go", issue.Position.File)
					assert.Contains(t, issue.Message, "calls api.openMemory, which opens sqlite")
					assert.Contains(t, issue.Message, "move it to api_integration_test.go with //go:build integration")
				case "TestStart":
					reason, _ := issue.Attr("reason")
					assert.Equal(t, "calls api.Start, which runs a process through exec.Command", reason)
				}
			}

			metric, ok := report.Metrics().Packages["./api"].(integration.Metric)
			require.True(t, ok, "metrics hold nothing for ./api")
			assert.Equal(t, integration.Metric{Tests: 7, Crossing: 5, Integration: 2}, metric)
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

// TestLintImportsAndStorage reads a document written by hand: a file that
// imports a driver and calls nothing, which crosses nothing, a storage path
// the tree names, a negated tag, and a tag the tree renames.
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
		integration.RuleBoundary: {"TestDial", "TestRepo"},
		integration.RuleFilename: {"slow_test.go"},
	}, byRule(issues))

	for _, issue := range issues {
		if issue.Symbol == "TestRepo" {
			reason, _ := issue.Attr("reason")
			assert.Equal(t, "imports example.com/app/repo and calls repo.Open", reason)
		}
	}
}

// TestLint_TestMain covers the testmain rule: a TestMain reaching a database,
// in a file of the build or behind the tag, makes its package an integration
// package; one that only runs a process does not, and the rule lets a
// package named for its tests be.
func TestLint_TestMain(t *testing.T) {
	t.Parallel()

	main := func(file string, refs model.StringSet) *model.Declaration {
		return &model.Declaration{Kind: model.FuncKind, Name: "TestMain", File: file, Line: 5, References: refs}
	}
	pkg := func(name, path, file string, refs model.StringSet, imports ...string) *model.Definition {
		return &model.Definition{
			Package: model.Package{Package: name + "_test", ImportPath: "example.com/" + path + "_test", Path: "./" + path, TestPackage: true},
			Files:   model.FileList{{Name: file, Test: true}},
			Imports: model.StringSet{file: imports},
			Funcs:   model.DeclarationList{main(file, refs)},
		}
	}
	open := model.StringSet{"sql": {"Open"}}
	root := &model.DocumentRoot{
		Packages: model.DefinitionList{
			pkg("store", "store", "main_test.go", open, `"database/sql"`),
			pkg("skillstest", "skillstest", "main_test.go", open, `"database/sql"`),
			pkg("e2e", "e2e", "main_test.go", open, `"database/sql"`),
			pkg("runner", "runner", "main_test.go", model.StringSet{"exec": {"Command"}}, `"os/exec"`),
		},
		Tagged: model.DefinitionList{func() *model.Definition {
			def := pkg("api", "api", "main_integration_test.go", open, `"database/sql"`)
			def.Excluded = model.StringSet{"main_integration_test.go": {"integration"}}
			return def
		}()},
	}

	_, issues := lint(t, enabled(), root)

	var files []string
	for _, issue := range issues {
		if issue.Rule != integration.RuleTestMain {
			continue
		}
		assert.Equal(t, model.SeverityError, issue.Severity)
		files = append(files, issue.Position.File)
	}
	assert.ElementsMatch(t, []string{"api/main_integration_test.go", "store/main_test.go"}, files)

	for _, issue := range issues {
		if issue.Rule == integration.RuleTestMain && issue.Position.File == "store/main_test.go" {
			assert.Equal(t, "TestMain opens a database through sql.Open: every test of store needs a database; move them behind //go:build integration, or name the package e2e or *test*", issue.Message)
		}
	}
}
