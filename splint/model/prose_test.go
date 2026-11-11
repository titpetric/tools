package model_test

import (
	"testing"

	"github.com/titpetric/tools/splint/model"
)

func TestAddProse(t *testing.T) {
	root := model.NewDocumentRoot(".", "simple")

	root.AddProse(nil)
	root.AddProse(&model.ProseFile{Path: "headings.md"})
	root.AddProse(&model.ProseFile{
		Path:   "docs/one.md",
		Blocks: []model.ProseBlock{{Line: 4, Text: "The pool reuses the frame."}},
	})

	if len(root.Prose) != 1 {
		t.Fatalf("got %d files, want the one holding a paragraph: %+v", len(root.Prose), root.Prose)
	}
	if root.Prose[0].Path != "docs/one.md" || root.Prose[0].Blocks[0].Line != 4 {
		t.Errorf("the file reads %+v", root.Prose[0])
	}
}
