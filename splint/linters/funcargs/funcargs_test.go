package funcargs

import (
	"context"
	"testing"

	"github.com/titpetric/tools/splint/model"
)

func TestFuncArgsLinter_ContextFirst(t *testing.T) {
	linter := New()
	defs := model.DefinitionList{
		{
			Package: model.Package{Path: "test"},
			Funcs: []*model.Declaration{
				{
					Name:      "Do",
					Kind:      model.FuncKind,
					Arguments: []string{"context.Context", "string"},
					File:      "test.go",
					Line:      1,
				},
			},
		},
	}
	report := lint(t, linter, defs)
	if report.Len() != 0 {
		t.Errorf("expected 0 issues for context.Context first, got %d", report.Len())
	}
}

func TestFuncArgsLinter_ContextNotFirst(t *testing.T) {
	linter := New()
	defs := model.DefinitionList{
		{
			Package: model.Package{Path: "test"},
			Funcs: []*model.Declaration{
				{
					Name:      "Render",
					Kind:      model.FuncKind,
					Arguments: []string{"int", "AnyContext"},
					File:      "test.go",
					Line:      1,
				},
			},
		},
	}
	report := lint(t, linter, defs)
	if report.Len() != 1 {
		t.Errorf("expected 1 issue for context not first, got %d", report.Len())
	}
}

func TestFuncArgsLinter_TimeDurationLast(t *testing.T) {
	linter := New()
	defs := model.DefinitionList{
		{
			Package: model.Package{Path: "test"},
			Funcs: []*model.Declaration{
				{
					Name:      "Set",
					Kind:      model.FuncKind,
					Arguments: []string{"string", "time.Duration"},
					File:      "test.go",
					Line:      1,
				},
			},
		},
	}
	report := lint(t, linter, defs)
	if report.Len() != 0 {
		t.Errorf("expected 0 issues for time.Duration last, got %d", report.Len())
	}
}

func TestFuncArgsLinter_TimeDurationNotLast(t *testing.T) {
	linter := New()
	defs := model.DefinitionList{
		{
			Package: model.Package{Path: "test"},
			Funcs: []*model.Declaration{
				{
					Name:      "Set",
					Kind:      model.FuncKind,
					Arguments: []string{"time.Duration", "string"},
					File:      "test.go",
					Line:      1,
				},
			},
		},
	}
	report := lint(t, linter, defs)
	if report.Len() != 1 {
		t.Errorf("expected 1 issue for time.Duration not last, got %d", report.Len())
	}
}

func TestFuncArgsLinter_DuplicateTypes(t *testing.T) {
	linter := New()
	defs := model.DefinitionList{
		{
			Package: model.Package{Path: "test"},
			Funcs: []*model.Declaration{
				{
					Name:      "Equal",
					Kind:      model.FuncKind,
					Arguments: []string{"*T", "*T"},
					File:      "test.go",
					Line:      1,
				},
			},
		},
	}
	report := lint(t, linter, defs)
	if report.Len() != 1 {
		t.Errorf("expected 1 issue for duplicate types, got %d", report.Len())
	}
}

func TestFuncArgsLinter_StringAnyAmbiguous(t *testing.T) {
	linter := New()
	defs := model.DefinitionList{
		{
			Package: model.Package{Path: "test"},
			Funcs: []*model.Declaration{
				{
					Name:      "Fetch",
					Kind:      model.FuncKind,
					Arguments: []string{"string", "map[string]any"},
					File:      "test.go",
					Line:      1,
				},
			},
		},
	}
	report := lint(t, linter, defs)
	if report.Len() != 0 {
		t.Errorf("expected 0 issues for (string, any) ambiguous, got %d", report.Len())
	}
}

func TestFuncArgsLinter_VariadicArguments(t *testing.T) {
	linter := New()
	defs := model.DefinitionList{
		{
			Package: model.Package{Path: "test"},
			Funcs: []*model.Declaration{
				{
					Name:      "Do",
					Kind:      model.FuncKind,
					Arguments: []string{"string", "...string"},
					File:      "test.go",
					Line:      1,
				},
			},
		},
	}
	report := lint(t, linter, defs)
	if report.Len() != 0 {
		t.Errorf("expected 0 issues for variadic arguments, got %d", report.Len())
	}
}

func TestFuncArgsLinter_InterfaceBeforeStruct(t *testing.T) {
	linter := New()
	defs := model.DefinitionList{
		{
			Package: model.Package{Path: "test"},
			Types: []*model.Declaration{
				{
					Name: "Reader",
					Type: "interface",
				},
			},
			Funcs: []*model.Declaration{
				{
					Name:      "Do",
					Kind:      model.FuncKind,
					Arguments: []string{"Reader", "*Config"},
					File:      "test.go",
					Line:      1,
				},
			},
		},
	}
	report := lint(t, linter, defs)
	if report.Len() != 0 {
		t.Errorf("expected 0 issues for interface before struct, got %d", report.Len())
	}
}

func TestFuncArgsLinter_StructBeforeInterface(t *testing.T) {
	linter := New()
	defs := model.DefinitionList{
		{
			Package: model.Package{Path: "test"},
			Types: []*model.Declaration{
				{
					Name: "Reader",
					Type: "interface",
				},
			},
			Funcs: []*model.Declaration{
				{
					Name:      "Do",
					Kind:      model.FuncKind,
					Arguments: []string{"*Config", "Reader"},
					File:      "test.go",
					Line:      1,
				},
			},
		},
	}
	report := lint(t, linter, defs)
	if report.Len() != 1 {
		t.Errorf("expected 1 issue for struct before interface, got %d", report.Len())
	}
}

func TestFuncArgsLinter_SingleArgumentPasses(t *testing.T) {
	linter := New()
	defs := model.DefinitionList{
		{
			Package: model.Package{Path: "test"},
			Funcs: []*model.Declaration{
				{
					Name:      "Process",
					Kind:      model.FuncKind,
					Arguments: []string{"context.Context"},
					File:      "test.go",
					Line:      1,
				},
			},
		},
	}
	report := lint(t, linter, defs)
	if report.Len() != 0 {
		t.Errorf("expected 0 issues for single argument, got %d", report.Len())
	}
}

// TestFuncArgsLinterOnlyConsidersTwoArguments covers the bound the linter
// draws: the order of one pair is unambiguous, and the expected order of three
// or more is a heuristic that reports too much to read.
func TestFuncArgsLinterOnlyConsidersTwoArguments(t *testing.T) {
	linter := New()
	defs := model.DefinitionList{
		{
			Package: model.Package{Path: "test"},
			Funcs: []*model.Declaration{
				{Name: "A", Kind: model.FuncKind, File: "test.go", Line: 1},
				{Name: "B", Kind: model.FuncKind, Arguments: []string{"time.Duration"}, File: "test.go", Line: 2},
				// Two arguments in the wrong order is the one case reported.
				{Name: "C", Kind: model.FuncKind, Arguments: []string{"time.Duration", "string"}, File: "test.go", Line: 3},
				// The same mistake with a third argument is not.
				{Name: "D", Kind: model.FuncKind, Arguments: []string{"time.Duration", "string", "int"}, File: "test.go", Line: 4},
			},
		},
	}

	report := lint(t, linter, defs)
	if report.Len() != 1 {
		t.Fatalf("expected 1 issue, got %d", report.Len())
	}
	for issue := range report.All() {
		if issue.Symbol != "C" {
			t.Errorf("reported %q, want the two argument func C", issue.Symbol)
		}
		if issue.Position.File != "test/test.go" || issue.Position.Line != 3 {
			t.Errorf("issue position = %q", issue.Position.Ref())
		}
	}
}

func TestFuncArgsLinter_MoreThanTwoArgumentsSkipped(t *testing.T) {
	linter := New()
	defs := model.DefinitionList{
		{
			Package: model.Package{Path: "test"},
			Types: []*model.Declaration{
				{
					Name: "Reader",
					Type: "interface",
				},
			},
			Funcs: []*model.Declaration{
				{
					Name:      "Do",
					Kind:      model.FuncKind,
					Arguments: []string{"context.Context", "*Config", "Reader"},
					File:      "test.go",
					Line:      1,
				},
				{
					Name:      "Set",
					Kind:      model.FuncKind,
					Arguments: []string{"context.Context", "time.Duration", "string"},
					File:      "test.go",
					Line:      2,
				},
			},
		},
	}
	report := lint(t, linter, defs)
	if report.Len() != 0 {
		t.Errorf("expected 0 issues for functions with 3 arguments, got %d", report.Len())
	}
}

// lint runs the linter over a list of packages, which is what every case here
// builds by hand.
func lint(t *testing.T, linter *Linter, defs model.DefinitionList) model.LintReport {
	t.Helper()

	report, err := linter.Lint(context.Background(), &model.DocumentRoot{Packages: defs})
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	return report
}
