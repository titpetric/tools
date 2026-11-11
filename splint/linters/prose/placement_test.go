package prose_test

import (
	"context"
	"testing"

	"github.com/titpetric/tools/splint/linters/prose"
	"github.com/titpetric/tools/splint/model"
)

func TestLinterReportsACommentBelowTheTopLevel(t *testing.T) {
	tests := []struct {
		title  string
		blocks []model.ProseBlock
		want   int
	}{
		{
			title:  "a comment above a top-level key",
			blocks: []model.ProseBlock{{Line: 1, Text: "The pipeline every change passes.", Comment: true}},
		},
		{
			title:  "a comment above a nested key",
			blocks: []model.ProseBlock{{Line: 12, Text: "The default pipeline skips this.", Comment: true, Depth: 1}},
			want:   1,
		},
		{
			title:  "a comment above a list item",
			blocks: []model.ProseBlock{{Line: 20, Text: "A test file cannot be split freely.", Comment: true, Depth: 3}},
			want:   1,
		},
		{
			title:  "a sentence-shaped value at any depth",
			blocks: []model.ProseBlock{{Line: 14, Text: "Format the Go source and the markdown", Depth: 2}},
		},
		{
			title: "one finding per nested comment",
			blocks: []model.ProseBlock{
				{Line: 1, Text: "The pipeline every change passes.", Comment: true},
				{Line: 12, Text: "The default pipeline skips this.", Comment: true, Depth: 1},
				{Line: 30, Text: "Both halves run under the protocol.", Comment: true, Depth: 2},
			},
			want: 2,
		},
	}

	for _, test := range tests {
		root := &model.DocumentRoot{Prose: []*model.ProseFile{{Path: "atkins.yml", Blocks: test.blocks}}}

		report, err := prose.New().Lint(context.Background(), root)
		if err != nil {
			t.Fatalf("%s: Lint() error = %v", test.title, err)
		}

		got := 0
		for _, issue := range model.Issues(report) {
			if issue.Rule != prose.RulePlacement {
				continue
			}
			got++
			if issue.Position.File != "atkins.yml" {
				t.Errorf("%s: file = %q, want atkins.yml", test.title, issue.Position.File)
			}
			if issue.Position.Line == 0 {
				t.Errorf("%s: the finding names no line", test.title)
			}
		}
		if got != test.want {
			t.Errorf("%s: %d placement findings, want %d", test.title, got, test.want)
		}
	}
}

func TestLinterReportsPlacementBesideTheMeasuredRules(t *testing.T) {
	root := &model.DocumentRoot{Prose: []*model.ProseFile{{
		Path:   "compose.yml",
		Blocks: []model.ProseBlock{{Line: 7, Text: "The service the fixtures query.", Comment: true, Depth: 2}},
	}}}

	found := 0
	for _, issue := range lint(t, root, baselineTree(t, "Open reads a file."), prose.Options{}) {
		if issue.Rule == prose.RulePlacement {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("%d placement findings, want the one a measured run reports beside its own", found)
	}
}
