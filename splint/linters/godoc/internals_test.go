package godoc_test

import (
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/linters/godoc"
	"github.com/titpetric/tools/splint/model"
)

func TestLinterReportsAnInternalCommentPastOneLine(t *testing.T) {
	pkg := model.Package{Package: "x", Path: "./x"}
	tests := []struct {
		title string
		name  string
		file  string
		doc   string
		rule  string
	}{
		{title: "one line names a rule", name: "open", file: "x.go", doc: "The caller holds the lock.", rule: ""},
		{title: "no comment at all", name: "open", file: "x.go", doc: "", rule: ""},
		{title: "two lines restate the body", name: "open", file: "x.go", doc: "open reads a file.\nIt returns the error the driver raised.", rule: godoc.RuleInternals},
		{title: "a blank line weighs nothing", name: "open", file: "x.go", doc: "The caller holds the lock.\n\n", rule: ""},
		{title: "an indented example weighs nothing", name: "open", file: "x.go", doc: "The caller holds the lock.\n\n\topen(f)", rule: ""},
		{title: "doc.go keeps its prose", name: "open", file: "doc.go", doc: "open reads a file.\nIt returns an error.", rule: ""},
		{title: "an exported symbol answers to the other rules", name: "Open", file: "x.go", doc: "Open reads a file.\nIt returns an error.", rule: ""},
	}

	for _, test := range tests {
		root := document(pkg, model.DeclarationList{
			{Kind: model.FuncKind, Name: test.name, File: test.file, Line: 3, Doc: test.doc,
				Complexity: &model.Complexity{Lines: 4, Blocks: 1}},
		}, nil)

		issues := lint(t, root)
		var got string
		for _, issue := range issues {
			if issue.Rule == godoc.RuleInternals {
				got = issue.Rule
				if !strings.Contains(issue.Message, "unexported symbol") {
					t.Errorf("%s: message = %q, want it to name the unexported symbol", test.title, issue.Message)
				}
			}
		}
		if got != test.rule {
			t.Errorf("%s: rule = %q, want %q (issues: %v)", test.title, got, test.rule, issues)
		}
	}
}
