package prose

import (
	"testing"
)

// counts is the skeletons of one block of prose.
func counts(text string, size int) *Counts {
	return Count(&Corpus{Blocks: []Block{{File: "a.go", Line: 1, Text: text}}}, size)
}

func TestMask(t *testing.T) {
	for token, want := range map[string]string{
		"the":    "the",
		"is":     "is",
		"beside": "beside",
		"frame":  "X",
		"pool":   "X",
	} {
		if got := mask(token); got != want {
			t.Errorf("mask(%q) is %q, want %q", token, got, want)
		}
	}
}

// Both sentences keep two function words inside one window of three. Spread
// them further apart and no shape survives at that size. The -n flag exists
// for that.
func TestCountShape(t *testing.T) {
	first := counts("The frame is what the pool reuses.", 3)
	second := counts("The panel is what the writer opens.", 3)

	for key := range first.Grams {
		if _, known := second.Grams[key]; !known {
			t.Errorf("shape %q is in one sentence and not the other", key)
		}
	}
	if len(first.Grams) == 0 {
		t.Fatal("no shapes at all")
	}
}

func TestCountFunctionWords(t *testing.T) {
	held := counts("Reflect evaluator tier dispatches shape tables.", 3)
	if len(held.Grams) != 0 {
		t.Errorf("got %d shapes from prose with no function words: %+v", len(held.Grams), held.Grams)
	}
}

func TestCountOneWord(t *testing.T) {
	held := counts("The helpers beside it.", 1)

	if _, known := held.Grams["beside"]; !known {
		t.Errorf("a one-word run has no shape for a preposition: %+v", held.Grams)
	}
	if _, known := held.Grams["X"]; known {
		t.Error("a one-word run counted a word of the subject matter")
	}
}

func TestCountPhrase(t *testing.T) {
	held := counts("The frame is what the pool reuses.", 3)

	gram, known := held.Grams["is what the"]
	if !known {
		t.Fatalf("no shape for the cleft: %+v", held.Grams)
	}
	if gram.Phrase != "is what the" || gram.File != "a.go" || gram.Line != 1 {
		t.Errorf("got %q at %s:%d", gram.Phrase, gram.File, gram.Line)
	}
}

func TestTokensSkipsSpans(t *testing.T) {
	got := tokens("The `every frame beside` holds a value.")
	for _, token := range got {
		switch token {
		case "every", "beside":
			t.Errorf("a code span reached the tokens: %q", got)
		}
	}
}

func TestTokensSkipsLinkTargets(t *testing.T) {
	got := tokens("See [the guide](docs/every-frame-beside.md).")
	for _, token := range got {
		if token == "beside" {
			t.Errorf("a link target reached the tokens: %q", got)
		}
	}
}
