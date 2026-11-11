package prose

import (
	"math"
	"sort"
	"strings"
)

// Density measures the shape of a corpus's sentences. A phrase list catches a
// tic. These catch the habit behind it.
type Density struct {
	// Blocks and Words tally the prose behind the rest.
	Blocks int
	Words  int

	// BlockWords is the median comment block, in words. A block covers one
	// symbol. This answers how far a function's comment runs.
	BlockWords int

	// SentenceWords is the median sentence. P90Words is the ninetieth
	// percentile, where the long ones live.
	SentenceWords int
	P90Words      int

	// Subordinators and Commas count per thousand words. A writer attaches a
	// reason once the sentence has said what a thing does. The comma marks
	// the join.
	Subordinators float64
	Commas        float64

	// Stacked is the share of blocks holding two subordinators or more: a
	// block explaining its own explanation.
	Stacked float64
}

// Measure reads the densities off a corpus.
func Measure(corpus *Corpus) Density {
	density := Density{Blocks: len(corpus.Blocks)}

	var (
		blockWords    []int
		sentenceWords []int
		subordinators int
		commas        int
		stacked       int
	)

	for _, block := range corpus.Blocks {
		count := len(tokens(block.Text))
		if count == 0 {
			continue
		}

		density.Words += count
		blockWords = append(blockWords, count)
		commas += strings.Count(block.Text, ",")

		for _, sentence := range sentences(block.Text) {
			if n := len(tokens(sentence)); n > 0 {
				sentenceWords = append(sentenceWords, n)
			}
		}

		found := 0
		for _, token := range tokens(block.Text) {
			if subordinator[token] {
				found++
			}
		}
		subordinators += found
		if found > 1 {
			stacked++
		}
	}

	density.BlockWords = median(blockWords)
	density.SentenceWords = median(sentenceWords)
	density.P90Words = percentile(sentenceWords, 0.9)
	density.Subordinators = per(subordinators, density.Words, 1000)
	density.Commas = per(commas, density.Words, 1000)
	density.Stacked = per(stacked, len(blockWords), 100)

	return density
}

// sentences splits a block the way a reader does. A block closing on no mark
// counts as one sentence: a label, or a list item nobody punctuated.
//
// An abbreviation's dot bounds nothing. The standard library writes "e.g."
// freely, and splitting there cuts its sentences in half. Every other number
// here stands against that baseline.
func sentences(text string) []string {
	var (
		out  []string
		from int
	)
	for _, at := range stop.FindAllStringIndex(text, -1) {
		if abbreviated(text[:at[0]+1]) {
			continue
		}
		out = append(out, text[from:at[1]])
		from = at[1]
	}

	if rest := strings.TrimSpace(text[from:]); rest != "" || len(out) == 0 {
		out = append(out, text[from:])
	}

	return out
}

// The short forms whose dot closes a word.
var abbreviations = words(`
e.g. i.e. etc. vs. cf. al. ca. approx. resp. viz. no. fig. eq. ch. sec.
mr. mrs. ms. dr. prof. st. jr. sr. inc. ltd. co.
`)

// abbreviated reports whether the text ends in a short form. A single letter
// counts. An initial ends in one. So does the first half of "e.g."
func abbreviated(text string) bool {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false
	}

	last := strings.ToLower(fields[len(fields)-1])
	if abbreviations[last] {
		return true
	}

	trimmed := strings.TrimSuffix(last, ".")
	return len(trimmed) == 1 && trimmed != "i" && trimmed != "a"
}

// median answers zero for an empty set.
func median(values []int) int {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)
	return sorted[len(sorted)/2]
}

// percentile is the value a share of the set falls at or below.
func percentile(values []int, share float64) int {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)
	at := int(math.Floor(share * float64(len(sorted))))
	if at >= len(sorted) {
		at = len(sorted) - 1
	}
	return sorted[at]
}

// per is a rate, and reads as zero where the whole is zero.
func per(part, whole, scale int) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole) * float64(scale)
}
