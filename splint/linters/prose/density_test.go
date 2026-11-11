package prose

import (
	"testing"
)

func measure(texts ...string) Density {
	corpus := &Corpus{}
	for i, text := range texts {
		corpus.Blocks = append(corpus.Blocks, Block{File: "a.go", Line: i + 1, Text: text})
	}
	return Measure(corpus)
}

func TestSentences(t *testing.T) {
	for text, want := range map[string]int{
		"The frame holds a value.":                     1,
		"The frame holds a value. The pool reuses it.": 2,
		"A label with no mark":                         1,
		"Go 1.27 holds the fix.":                       1,
		"The tier, e.g. the bridge, packs the call.":   1,
	} {
		if got := len(sentences(text)); got != want {
			t.Errorf("%q split into %d sentences, want %d: %q", text, got, want, sentences(text))
		}
	}
}

// "If" and "when" stay off the list. An API comment states a contract, and
// those two words are how it states one.
func TestSubordinators(t *testing.T) {
	reason := measure("The pool reuses the frame because the call is hot, which keeps it warm.")
	if reason.Subordinators == 0 {
		t.Error("a sentence with two reasons counted no subordinators")
	}
	if reason.Stacked != 100 {
		t.Errorf("a block stacking two subordinators is %.0f%% of the blocks, want 100", reason.Stacked)
	}

	contract := measure("If n is negative, Read answers an error when the context ends.")
	if contract.Subordinators != 0 {
		t.Errorf("a contract counted %.2f subordinators per 1000 words, want none", contract.Subordinators)
	}
}

func TestDensityBlocks(t *testing.T) {
	got := measure("One two three four five.", "One two. Three four five six seven eight.")

	if got.Blocks != 2 {
		t.Errorf("got %d blocks, want 2", got.Blocks)
	}
	if got.Words != 13 {
		t.Errorf("got %d words, want 13", got.Words)
	}
	if got.BlockWords != 8 {
		t.Errorf("median words per block is %d, want 8", got.BlockWords)
	}
	if got.SentenceWords != 5 {
		t.Errorf("median words per sentence is %d, want 5", got.SentenceWords)
	}
}

func TestDensityEmpty(t *testing.T) {
	got := measure()

	if got.Blocks != 0 || got.BlockWords != 0 || got.Subordinators != 0 || got.Stacked != 0 {
		t.Errorf("an empty corpus measured %+v", got)
	}
}

func TestPercentile(t *testing.T) {
	values := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	if got := percentile(values, 0.9); got != 10 {
		t.Errorf("p90 is %d, want 10", got)
	}
	if got := percentile(nil, 0.9); got != 0 {
		t.Errorf("p90 of nothing is %d, want 0", got)
	}
}
