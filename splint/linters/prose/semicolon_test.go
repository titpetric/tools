package prose

import (
	"context"
	"testing"

	"github.com/titpetric/tools/splint/model"
)

func TestSemicolonFindings(t *testing.T) {
	tests := []struct {
		title string
		text  string
		want  int
	}{
		{
			title: "a semicolon between two clauses",
			text:  "The gate runs fast; the suite runs in CI.",
			want:  1,
		},
		{
			title: "a sentence without one",
			text:  "The gate runs fast. The suite runs in CI.",
		},
		{
			title: "a code span",
			text:  "The script runs `a; b` as one line.",
		},
		{
			title: "a link target",
			text:  "The [guide](https://example.com/a;b) names the flag.",
		},
		{
			title: "one finding per block, however many semicolons",
			text:  "One; two; three.",
			want:  1,
		},
	}

	for _, test := range tests {
		files := []*model.ProseFile{{
			Path:   "docs/guide.md",
			Blocks: []model.ProseBlock{{Line: 7, Text: test.text}},
		}}

		got := semicolonFindings(files)
		if len(got) != test.want {
			t.Errorf("%s: %d findings, want %d", test.title, len(got), test.want)
			continue
		}
		for _, one := range got {
			if one.Rule != RuleSemicolon {
				t.Errorf("%s: rule = %q, want %q", test.title, one.Rule, RuleSemicolon)
			}
			if one.Position.File != "docs/guide.md" || one.Position.Line != 7 {
				t.Errorf("%s: position = %s:%d, want docs/guide.md:7", test.title, one.Position.File, one.Position.Line)
			}
		}
	}
}

func TestLinterReportsSemicolonWithNoBaseline(t *testing.T) {
	root := &model.DocumentRoot{Prose: []*model.ProseFile{{
		Path:   "docs/guide.md",
		Blocks: []model.ProseBlock{{Line: 3, Text: "The gate runs fast; the suite runs in CI."}},
	}}}

	report, err := New().Lint(context.Background(), root)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	found := 0
	for _, issue := range model.Issues(report) {
		if issue.Rule == RuleSemicolon {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("%d semicolon findings, want the one a run with no baseline reports", found)
	}
}
