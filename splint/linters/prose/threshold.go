package prose

import (
	"fmt"

	"github.com/titpetric/tools/splint/model"
)

// slack is how far over the baseline a density runs before the linter says
// so.
//
// The figure clears 1.0. The baseline is one corpus and a tree is another.
// Two bodies of honest prose differ by a quarter on any of these with neither
// one wrong. Half again marks a habit.
const slack = 1.5

// densityFindings reports the measures of sentence shape running above the
// baseline.
//
// These are the half of the measure a rewrite cannot dodge. A banned phrase
// has a synonym. A sentence of thirty words has no synonym of ten. So the
// message names both numbers and leaves the wording to the writer.
func densityFindings(tree, baseline Density) []Result {
	var found []Result

	for _, one := range []struct {
		name     string
		tree     float64
		baseline float64
		unit     string
		advice   string
	}{
		{
			"a comment block", float64(tree.BlockWords), float64(baseline.BlockWords), "words",
			"A block covers one symbol. Cut what the signature already says.",
		},
		{
			"a sentence", float64(tree.SentenceWords), float64(baseline.SentenceWords), "words",
			"Break it where the subject changes.",
		},
		{
			"the longest tenth of sentences", float64(tree.P90Words), float64(baseline.P90Words), "words",
			"Break them where the subject changes.",
		},
		{
			"a clause carrying a reason", tree.Subordinators, baseline.Subordinators, "per 1000 words",
			"State the fact. Give the reason its own sentence, or drop it.",
		},
		{
			"a block stacking two reasons or more", tree.Stacked, baseline.Stacked, "blocks in a hundred",
			"A block explaining its own explanation says neither thing.",
		},
	} {
		if one.baseline <= 0 || one.tree <= one.baseline*slack {
			continue
		}

		found = append(found, Result{
			Rule:    RuleDensity,
			Message: densityMessage(one.name, one.tree, one.baseline, one.unit, one.advice),
			// No file and no line: the measure is of the tree.
			Position: model.Position{},
		})
	}

	return found
}

// densityMessage states the measure, the baseline and the margin, then what
// to do about it.
func densityMessage(name string, got, want float64, unit, advice string) string {
	return fmt.Sprintf("%s runs to %s %s against the baseline's %s, %.1fx. %s",
		name, trim(got), unit, trim(want), got/want, advice)
}

// trim writes a measure without a trailing zero. A count of words then reads
// as a count and a rate reads as a rate.
func trim(value float64) string {
	if value == float64(int(value)) {
		return fmt.Sprintf("%d", int(value))
	}
	return fmt.Sprintf("%.2f", value)
}
