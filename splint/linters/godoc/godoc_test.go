package godoc_test

import (
	"context"
	"testing"

	"github.com/titpetric/tools/splint/linters/godoc"
	"github.com/titpetric/tools/splint/model"
)

// document wraps a list of declarations into the document a linter reads.
func document(pkg model.Package, funcs, consts model.DeclarationList) *model.DocumentRoot {
	return &model.DocumentRoot{Packages: model.DefinitionList{{
		Package: pkg,
		Funcs:   funcs,
		Consts:  consts,
	}}}
}

// lint runs the linter and returns the issues it found.
func lint(t *testing.T, root *model.DocumentRoot) []model.Issue {
	t.Helper()

	report, err := godoc.New().Lint(context.Background(), root)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if report.Linter() != godoc.Name {
		t.Errorf("Linter() = %q, want %q", report.Linter(), godoc.Name)
	}

	issues := model.Issues(report)
	if len(issues) != report.Len() {
		t.Errorf("Len() = %d, walked %d", report.Len(), len(issues))
	}
	return issues
}

func TestLinterReportsWhatIsWrongWithADoc(t *testing.T) {
	pkg := model.Package{Package: "x", Path: "./x"}
	tests := []struct {
		title string
		doc   string
		rule  string
	}{
		{title: "no doc at all", doc: "", rule: godoc.RuleMissing},
		{title: "opens on the wrong word", doc: "This opens wrong.", rule: godoc.RuleFormat},
		{title: "ends on nothing", doc: "Open reads a file", rule: godoc.RuleFormat},
		{title: "as it should be", doc: "Open reads a file.", rule: ""},
		{title: "a code span ends it", doc: "Open reads `a file`", rule: ""},
	}

	for _, test := range tests {
		root := document(pkg, model.DeclarationList{
			{Kind: model.FuncKind, Name: "Open", File: "x.go", Line: 3, Doc: test.doc},
		}, nil)

		issues := lint(t, root)
		switch {
		case test.rule == "" && len(issues) > 0:
			t.Errorf("%s: reported %q", test.title, issues[0].Message)
		case test.rule != "" && len(issues) != 1:
			t.Errorf("%s: reported %d issues, want 1", test.title, len(issues))
		case test.rule != "":
			if issues[0].Rule != test.rule {
				t.Errorf("%s: rule = %q, want %q", test.title, issues[0].Rule, test.rule)
			}
			if issues[0].Position.Ref() != "x/x.go:3" {
				t.Errorf("%s: position = %q", test.title, issues[0].Position.Ref())
			}
			if issues[0].Symbol != "Open" {
				t.Errorf("%s: symbol = %q", test.title, issues[0].Symbol)
			}
		}
	}
}

// TestLinterReadsAConstBlock covers the two shapes a set of related constants
// arrives in.
//
// A parenthesised block is one declaration naming several symbols, and the
// comment above it documents the block rather than any one name, so the name
// check does not apply to it. A run of separate declarations is a block by
// proximity, and a comment on the first of them carries to the rest.
func TestLinterReadsAConstBlock(t *testing.T) {
	pkg := model.Package{Package: "x", Path: "."}

	// One declaration, several names: the comment names none of them.
	root := document(pkg, nil, model.DeclarationList{
		{Kind: model.ConstKind, Names: []string{"KindA", "KindB"}, File: "x.go", Line: 3, Doc: "The kinds of a thing."},
	})
	if issues := lint(t, root); len(issues) != 0 {
		t.Errorf("reported %d issues on a documented block: %q", len(issues), issues[0].Message)
	}

	// The same block with no punctuation is still checked for that.
	root = document(pkg, nil, model.DeclarationList{
		{Kind: model.ConstKind, Names: []string{"KindA", "KindB"}, File: "x.go", Line: 3, Doc: "The kinds of a thing"},
	})
	if issues := lint(t, root); len(issues) != 1 || issues[0].Rule != godoc.RuleFormat {
		t.Errorf("reported %#v, want one format issue", issues)
	}

	// Separate declarations close together: the first documents the run.
	root = document(pkg, nil, model.DeclarationList{
		{Kind: model.ConstKind, Name: "KindA", File: "x.go", Line: 3, Doc: "KindA is the first."},
		{Kind: model.ConstKind, Name: "KindB", File: "x.go", Line: 4},
		{Kind: model.ConstKind, Name: "KindC", File: "x.go", Line: 5},
	})
	if issues := lint(t, root); len(issues) != 0 {
		t.Errorf("reported %d issues under an opening comment: %q", len(issues), issues[0].Message)
	}

	// A gap wide enough to be another block is another block, and that one
	// carries no comment of its own.
	root = document(pkg, nil, model.DeclarationList{
		{Kind: model.ConstKind, Name: "KindA", File: "x.go", Line: 3, Doc: "KindA is the first."},
		{Kind: model.ConstKind, Name: "KindB", File: "x.go", Line: 40},
	})
	if issues := lint(t, root); len(issues) != 1 || issues[0].Rule != godoc.RuleMissing {
		t.Errorf("reported %#v, want one missing issue across the gap", issues)
	}
}

// TestLinterSkipsWhatNobodyReads covers the two scopes a doc comment is not a
// promise in.
func TestLinterSkipsWhatNobodyReads(t *testing.T) {
	undocumented := model.DeclarationList{{Kind: model.FuncKind, Name: "Open", File: "x.go", Line: 1}}

	for _, pkg := range []model.Package{
		{Package: "main", Path: "./cmd/x"},
		{Package: "x", Path: "./x", TestPackage: true},
	} {
		if issues := lint(t, document(pkg, undocumented, nil)); len(issues) != 0 {
			t.Errorf("%s: reported %d issues", pkg.Package, len(issues))
		}
	}

	// An unexported symbol is nobody's business either.
	internal := model.DeclarationList{{Kind: model.FuncKind, Name: "open", File: "x.go", Line: 1}}
	if issues := lint(t, document(model.Package{Package: "x"}, internal, nil)); len(issues) != 0 {
		t.Errorf("reported %d issues on an unexported symbol", len(issues))
	}
}

func TestLinterName(t *testing.T) {
	if got := godoc.New().Name(); got != godoc.Name {
		t.Errorf("Name() = %q, want %q", got, godoc.Name)
	}
}
