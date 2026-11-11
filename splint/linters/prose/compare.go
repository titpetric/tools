package prose

import (
	"math"
	"sort"
)

// Compare ranks the shapes a tree over-uses, by log-likelihood (see docs.go).
func Compare(tree, baseline *Counts, limits Limits) []Finding {
	var found []Finding

	for key, gram := range tree.Grams {
		if gram.Count < limits.Count {
			continue
		}

		seen := 0
		if other, known := baseline.Grams[key]; known {
			seen = other.Count
		}

		rate := per(gram.Count, tree.Words, 1)
		other := per(seen, baseline.Words, 1)
		if other > 0 && rate/other < limits.Ratio {
			continue
		}

		score := likelihood(gram.Count, seen, tree.Words, baseline.Words)
		if score < limits.Score {
			continue
		}

		ratio := 0.0
		if other > 0 {
			ratio = rate / other
		}

		found = append(found, Finding{
			Skeleton: key,
			Phrase:   gram.Phrase,
			File:     gram.File,
			Line:     gram.Line,
			Count:    gram.Count,
			Baseline: seen,
			Ratio:    ratio,
			Absent:   seen == 0,
			Score:    score,
		})
	}

	sort.Slice(found, func(i, j int) bool {
		left, right := found[i], found[j]
		if left.Score != right.Score {
			return left.Score > right.Score
		}
		if left.Count != right.Count {
			return left.Count > right.Count
		}
		return left.Skeleton < right.Skeleton
	})

	return found
}

// likelihood takes Dunning's log-likelihood over the two counts and the two
// corpus sizes. It measures the distance from the split one shared rate would
// produce.
//
// A count of zero contributes nothing. That falls out as the limit of
// x*log(x) at zero, with no case to handle.
func likelihood(count, seen, words, baselineWords int) float64 {
	if words == 0 || baselineWords == 0 {
		return 0
	}

	total := float64(count + seen)
	size := float64(words) + float64(baselineWords)
	expected := float64(words) * total / size
	expectedBaseline := float64(baselineWords) * total / size

	score := 0.0
	if count > 0 && expected > 0 {
		score += float64(count) * math.Log(float64(count)/expected)
	}
	if seen > 0 && expectedBaseline > 0 {
		score += float64(seen) * math.Log(float64(seen)/expectedBaseline)
	}

	return 2 * score
}
