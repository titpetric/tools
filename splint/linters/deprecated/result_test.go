package deprecated_test

import (
	"testing"

	"github.com/titpetric/tools/splint/linters/deprecated"
	"github.com/titpetric/tools/splint/model"
)

// A use renders as an error naming the symbol at the line it is written on.
func TestResult(t *testing.T) {
	result := deprecated.Result{
		Symbol:   "Old",
		Position: model.Position{File: "app/app.go", Line: 6},
		Message:  "uses deprecated Old: use New.",
	}
	issue := result.Issue()

	if issue.Linter != deprecated.Name || issue.Rule != deprecated.RuleUse {
		t.Errorf("issue = %s/%s, want %s/%s", issue.Linter, issue.Rule, deprecated.Name, deprecated.RuleUse)
	}
	if issue.Severity != model.SeverityError {
		t.Errorf("severity = %v, want error", issue.Severity)
	}
	if issue.Position.Ref() != "app/app.go:6" {
		t.Errorf("position = %q", issue.Position.Ref())
	}
	if issue.Symbol != "Old" || issue.Message != "uses deprecated Old: use New." {
		t.Errorf("issue = %#v", issue)
	}
}
