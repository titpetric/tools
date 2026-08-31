package funcreturns_test

import (
	"context"
	"testing"

	"github.com/titpetric/tools/splint/linters/funcreturns"
	"github.com/titpetric/tools/splint/model"
)

// lint runs the linter over one function and returns what it found.
func lint(t *testing.T, returns []string) []model.Issue {
	t.Helper()

	root := &model.DocumentRoot{Packages: model.DefinitionList{{
		Package: model.Package{Package: "x", Path: "./x"},
		Funcs: model.DeclarationList{{
			Kind: model.FuncKind, Name: "Do", File: "x.go", Line: 7, Returns: returns,
		}},
	}}}

	report, err := funcreturns.New().Lint(context.Background(), root)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	return model.Issues(report)
}

func TestLinterReadsReturnOrder(t *testing.T) {
	tests := []struct {
		returns []string
		want    bool
	}{
		// The value first, then the bool that says whether it is there, then
		// the error that says why it is not.
		{returns: []string{"User", "error"}, want: false},
		{returns: []string{"User", "bool"}, want: false},
		{returns: []string{"User", "bool", "error"}, want: false},
		{returns: []string{"error", "User"}, want: true},
		{returns: []string{"bool", "User"}, want: true},
		{returns: []string{"error", "bool"}, want: true},
		// One result or none has no order to get wrong.
		{returns: []string{"error"}, want: false},
		{returns: nil, want: false},
	}

	for _, test := range tests {
		issues := lint(t, test.returns)
		if got := len(issues) > 0; got != test.want {
			t.Errorf("%v: reported %v, want %v", test.returns, got, test.want)
		}
		if !test.want {
			continue
		}
		if issues[0].Position.Ref() != "x/x.go:7" {
			t.Errorf("%v: position = %q", test.returns, issues[0].Position.Ref())
		}
		if issues[0].Symbol != "Do" {
			t.Errorf("%v: symbol = %q", test.returns, issues[0].Symbol)
		}
		if got, _ := issues[0].Attr("expected"); got == "" {
			t.Errorf("%v: the issue does not say what the order should be", test.returns)
		}
	}
}

func TestLinterSkipsTestScope(t *testing.T) {
	root := &model.DocumentRoot{Packages: model.DefinitionList{{
		Package: model.Package{Package: "x"},
		Funcs: model.DeclarationList{{
			Kind: model.FuncKind, Name: "Do", File: "x_test.go", Returns: []string{"error", "User"},
		}},
	}}}

	report, err := funcreturns.New().Lint(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if report.Len() != 0 {
		t.Errorf("reported %d issues in a test file", report.Len())
	}
}
