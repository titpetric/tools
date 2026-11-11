package prose

import (
	"testing"

	"github.com/titpetric/tools/splint/model"
)

func TestFromDocumentReadsBothKinds(t *testing.T) {
	root := &model.DocumentRoot{
		Packages: model.DefinitionList{{
			Package: model.Package{Package: "x", Path: "./x"},
			Doc:     "Package x does a thing.",
			Funcs: model.DeclarationList{
				{File: "x/a.go", Line: 7, Doc: "A returns a value."},
				{File: "x/b.go", Line: 3},
			},
		}},
		Prose: []*model.ProseFile{{
			Path:   "docs/one.md",
			Blocks: []model.ProseBlock{{Line: 4, Text: "The pool reuses the frame."}},
		}},
	}

	corpus := FromDocument(root)
	if len(corpus.Blocks) != 3 {
		t.Fatalf("got %d blocks, want 3: %+v", len(corpus.Blocks), corpus.Blocks)
	}

	docs := corpus.Docs()
	if len(docs.Blocks) != 2 {
		t.Errorf("Docs() holds %d blocks, want the two comments: %+v", len(docs.Blocks), docs.Blocks)
	}
	for _, block := range docs.Blocks {
		if block.Prose {
			t.Errorf("Docs() kept a markdown block: %+v", block)
		}
	}
}

func TestFromDocumentSkipsWhatIsNotWritten(t *testing.T) {
	root := &model.DocumentRoot{Packages: model.DefinitionList{{
		Package: model.Package{Package: "x", Path: "./x"},
		Funcs: model.DeclarationList{
			{File: "x/a.go", Line: 1},
			{File: "x/b.go", Line: 2, Doc: "   \n\t\n"},
		},
	}}}

	if corpus := FromDocument(root); len(corpus.Blocks) != 0 {
		t.Errorf("got %d blocks, want none: %+v", len(corpus.Blocks), corpus.Blocks)
	}
}

// An indented line in a doc comment is an example, which a generated
// reference prints as code. A run of identifiers is not a sentence.
func TestAddLeavesAnExampleOut(t *testing.T) {
	corpus := &Corpus{}
	corpus.add("a.go", 1, "Open reads the file.\n\n\tf, err := Open(path)\n\tdefer f.Close()\n\nThe caller closes it.", false)

	if len(corpus.Blocks) != 1 {
		t.Fatalf("got %d blocks, want 1: %+v", len(corpus.Blocks), corpus.Blocks)
	}
	if got, want := corpus.Blocks[0].Text, "Open reads the file. The caller closes it."; got != want {
		t.Errorf("the block reads %q, want %q", got, want)
	}
}

func TestDocsOfNothing(t *testing.T) {
	if docs := (&Corpus{}).Docs(); len(docs.Blocks) != 0 {
		t.Errorf("Docs() of an empty corpus holds %d blocks", len(docs.Blocks))
	}
}
