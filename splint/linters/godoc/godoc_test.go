package godoc_test

import (
	"context"
	"strings"
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
// A parenthesised block is one declaration naming many symbols. The
// comment above it documents the block and no single name. The name
// check does not apply to it. A run of separate declarations is a block by
// proximity. A comment on the first of them holds to the rest.
func TestLinterReadsAConstBlock(t *testing.T) {
	pkg := model.Package{Package: "x", Path: "."}

	// One declaration, two names: the comment names none of them.
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

	// A gap wide enough to be another block is another block. That one
	// holds no comment at all.
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

// TestLinterSkipsAGeneratedFile covers the one undocumented symbol nobody in the
// tree can document: the comment on a generated declaration is the generator's
// to write. A finding on it names work nobody here can do.
func TestLinterSkipsAGeneratedFile(t *testing.T) {
	root := document(model.Package{Package: "x", Path: "./x"},
		model.DeclarationList{
			{Kind: model.FuncKind, Name: "Sort", File: "sort_i64.go", Line: 6},
			{Kind: model.FuncKind, Name: "Open", File: "x.go", Line: 1},
		}, nil)
	root.Packages[0].Files = model.FileList{
		{Name: "sort_i64.go", Generated: true},
		{Name: "x.go"},
	}

	issues := lint(t, root)
	if len(issues) != 1 {
		t.Fatalf("reported %d issues, want 1: %#v", len(issues), issues)
	}
	if issues[0].Symbol != "Open" {
		t.Errorf("issue is about %q, want the hand-written file's symbol", issues[0].Symbol)
	}
}

// TestLinterWeighsADocAgainstItsBlocks covers the function comment running to
// more lines than the body opens blocks, which belongs on the type or in
// docs.go.
func TestLinterWeighsADocAgainstItsBlocks(t *testing.T) {
	pkg := model.Package{Package: "x", Path: "./x"}
	tests := []struct {
		title  string
		file   string
		doc    string
		blocks int
		body   bool
		rule   string
	}{
		{title: "one line over no block", doc: "Open reads a file.", blocks: 0, body: true, rule: ""},
		{title: "two lines over no block", doc: "Open reads\na file.", blocks: 0, body: true, rule: godoc.RuleOutweighs},
		{title: "two lines over one block", doc: "Open reads\na file.", blocks: 1, body: true, rule: ""},
		{title: "three lines over one block", doc: "Open reads\na file\nby name.", blocks: 1, body: true, rule: godoc.RuleOutweighs},
		{title: "five lines over four blocks", doc: "Open is\na long\nexplanation\nof very\nlittle.", blocks: 4, body: true, rule: ""},
		{title: "five lines over two blocks", doc: "Open is\na long\nexplanation\nof very\nlittle.", blocks: 2, body: true, rule: godoc.RuleOutweighs},
		{title: "eleven lines is verbose first", doc: "Open\nis\neleven\nlines\nover\none\nand\nsays\nso\nbefore\nthis.", blocks: 1, body: true, rule: godoc.RuleVerbose},
		{title: "a type measures nothing", doc: "Open is\na type,\nwith no body\nfor a comment\nto outweigh.", rule: ""},
		{title: "docs.go measures nothing", file: "docs.go", doc: "Open reads\na file.", blocks: 0, body: true, rule: ""},
		{title: "an example weighs nothing", doc: "Open reads\na file:\n\n\tf := Open()\n\nThe caller closes it.", blocks: 2, body: true, rule: ""},
	}

	for _, test := range tests {
		file := test.file
		if file == "" {
			file = "x.go"
		}
		decl := &model.Declaration{Kind: model.FuncKind, Name: "Open", File: file, Line: 9, Doc: test.doc}
		if test.body {
			decl.Complexity = &model.Complexity{Lines: 5, Blocks: test.blocks}
		}

		issues := lint(t, document(pkg, model.DeclarationList{decl}, nil))
		switch {
		case test.rule == "" && len(issues) > 0:
			t.Errorf("%s: reported %q", test.title, issues[0].Message)
		case test.rule != "" && len(issues) != 1:
			t.Errorf("%s: reported %d issues, want 1", test.title, len(issues))
		case test.rule != "" && issues[0].Rule != test.rule:
			t.Errorf("%s: rule = %q, want %q", test.title, issues[0].Rule, test.rule)
		}
	}

	decl := &model.Declaration{Kind: model.FuncKind, Name: "Open", File: "x.go", Line: 9, Doc: "Open reads\na file\nand more\nof it.", Complexity: &model.Complexity{Lines: 9, Blocks: 1}}
	issues := lint(t, document(pkg, model.DeclarationList{decl}, nil))
	want := "godoc runs to 4 prose lines against a budget of 2 lines, one for the function and one per block it opens; cut it, or put the intent on the type or in doc.go"
	if len(issues) != 1 || issues[0].Message != want {
		t.Errorf("reported %#v, want %q", issues, want)
	}
}

func TestLinterBrevity(t *testing.T) {
	pkg := model.Package{Package: "x", ImportPath: "x"}
	two := "Open reads\na file."
	decl := func() *model.Declaration {
		return &model.Declaration{Kind: model.FuncKind, Name: "Open", File: "x.go", Line: 9, Doc: two, Complexity: &model.Complexity{Lines: 9, Blocks: 3}}
	}

	// Four blocks earn four lines, so two lines pass the default budget.
	if issues := lint(t, document(pkg, model.DeclarationList{decl()}, nil)); len(issues) != 0 {
		t.Errorf("default budget reported %#v, want none", issues)
	}

	linter := godoc.New()
	linter.Brevity = true
	report, err := linter.Lint(context.Background(), document(pkg, model.DeclarationList{decl()}, nil))
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	issues := model.Issues(report)
	if len(issues) != 1 || issues[0].Rule != godoc.RuleOutweighs {
		t.Fatalf("brevity reported %#v, want one outweighs", issues)
	}
	if !strings.Contains(issues[0].Message, "budget of one") || !strings.Contains(issues[0].Message, "docs/") {
		t.Errorf("message = %q, want the budget and where the prose goes", issues[0].Message)
	}

	// One line is the whole budget, and it passes.
	one := decl()
	one.Doc = "Open reads a file."
	linter2 := godoc.New()
	linter2.Brevity = true
	report2, err := linter2.Lint(context.Background(), document(pkg, model.DeclarationList{one}, nil))
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if got := model.Issues(report2); len(got) != 0 {
		t.Errorf("one line reported %#v, want none", got)
	}
}

func TestLinterName(t *testing.T) {
	if got := godoc.New().Name(); got != godoc.Name {
		t.Errorf("Name() = %q, want %q", got, godoc.Name)
	}
}
