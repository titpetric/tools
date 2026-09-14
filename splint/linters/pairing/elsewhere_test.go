package pairing_test

import (
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/model"
)

// elsewhere builds a directory of one file and one exported symbol, and a
// test in another package that may or may not reference it.
func elsewhere(referenced string) *model.DocumentRoot {
	subject := definition(
		model.Package{Package: "a", ImportPath: "example.com/a", Path: "./a"},
		model.File{Name: "config.go"},
	)
	subject.Funcs = model.DeclarationList{
		{Kind: model.FuncKind, Name: "Load", File: "config.go", Line: 3},
	}

	test := definition(
		model.Package{Package: "b_test", ImportPath: "example.com/b_test", Path: "./b", TestPackage: true},
		model.File{Name: "b_test.go", Test: true},
	)
	test.Imports = model.StringSet{"b_test.go": []string{`"example.com/a"`}}
	test.Funcs = model.DeclarationList{
		{
			Kind:       model.FuncKind,
			Name:       "TestLoad",
			File:       "b_test.go",
			Line:       1,
			References: model.StringSet{"a": []string{referenced}},
		},
	}

	return document(subject, test)
}

// TestLinter_Lint_TestedElsewhere covers the excuse: a standalone file whose
// exported surface is referenced by a test in another package is counted
// rather than reported, and one whose surface is not stays a finding.
func TestLinter_Lint_TestedElsewhere(t *testing.T) {
	issues, report := lint(t, elsewhere("Load"))
	for _, issue := range issues {
		if issue.Position.File == "a/config.go" {
			t.Errorf("config.go is reported although a test references Load: %v", issue)
		}
	}
	if want := "with 1 file tested from another package"; !contains(report, want) {
		t.Errorf("the footer does not say %q", want)
	}

	issues, _ = lint(t, elsewhere("Other"))
	found := false
	for _, issue := range issues {
		if issue.Position.File == "a/config.go" {
			found = true
		}
	}
	if !found {
		t.Error("config.go is not reported although nothing references Load")
	}
}

// contains reports the footer of the one table saying something.
func contains(report model.LintReport, want string) bool {
	for _, table := range report.Statistics() {
		if strings.Contains(table.Footer, want) {
			return true
		}
	}
	return false
}
