package imports_test

import (
	"context"
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/linters/imports"
	"github.com/titpetric/tools/splint/model"
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
