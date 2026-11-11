package prose

import (
	"strings"
)

// Counts holds every skeleton of one corpus, beside its word total.
type Counts struct {
	Grams map[string]*Gram
	Words int
}

// Count builds the skeleton n-grams of a corpus (see docs.go).
func Count(corpus *Corpus, size int) *Counts {
	counts := &Counts{Grams: map[string]*Gram{}}

	for _, block := range corpus.Blocks {
		tokens := tokens(block.Text)
		counts.Words += len(tokens)

		shape := make([]string, len(tokens))
		for i, token := range tokens {
			shape[i] = mask(token)
		}

		for i := 0; i+size <= len(shape); i++ {
			skeleton := shape[i : i+size]
			if size-content(skeleton) < min(2, size) {
				continue
			}

			key := strings.Join(skeleton, " ")
			gram, known := counts.Grams[key]

			// A package comment names no line. A finding naming a package
			// and no line sends a reader nowhere. The first instance naming a
			// line wins over one naming none.
			if known && gram.Line == 0 && block.Line > 0 {
				gram.File, gram.Line = block.File, block.Line
				gram.Phrase = strings.Join(tokens[i:i+size], " ")
			}

			if !known {
				gram = &Gram{
					Skeleton: key,
					File:     block.File,
					Line:     block.Line,
					Phrase:   strings.Join(tokens[i:i+size], " "),
				}
				counts.Grams[key] = gram
			}
			gram.Count++
		}
	}

	return counts
}
