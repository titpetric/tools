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

// TestFuncArgsLinterReadsTheParameterCount covers the difference between the
// two numbers a declaration carries. Arguments holds the distinct types and no
// duplicate, so time.Date(year int, month time.Month, day, hour, min, sec,
// nsec int) records two of them for seven parameters. Reading that list's
// length as the arity made the linter report the pair reversed, which would
// put the month before the year in a function that mirrors time.Date
// one for one.
func TestFuncArgsLinterReadsTheParameterCount(t *testing.T) {
	linter := New()
	defs := model.DefinitionList{
		{
			Package: model.Package{Path: "test"},
			Funcs: []*model.Declaration{
				{
					Name:          "Date",
					Kind:          model.FuncKind,
					Arguments:     []string{"int", "time.Month"},
					ArgumentCount: 7,
					File:          "test.go",
					Line:          1,
				},
			},
		},
	}
	report := lint(t, linter, defs)
	if report.Len() != 0 {
		t.Errorf("expected 0 issues for a seven parameter func, got %d", report.Len())
	}
}

// lint runs the linter over a list of packages, the shape every case here
// builds by hand.
//
// A declaration that states no ArgumentCount gets one parameter per type in
// Arguments, which matches what every case above means. A case where the two differ
// says so itself: the linter skips a declaration whose count is zero, so
// filling it here keeps an expectation of no findings from passing
// on a run that read nothing.
func lint(t *testing.T, linter *Linter, defs model.DefinitionList) model.LintReport {
	t.Helper()

	for _, def := range defs {
		for _, decl := range def.Funcs {
			if decl.ArgumentCount == 0 {
				decl.ArgumentCount = len(decl.Arguments)
			}
		}
	}

	report, err := linter.Lint(context.Background(), &model.DocumentRoot{Packages: defs})
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	return report
}
