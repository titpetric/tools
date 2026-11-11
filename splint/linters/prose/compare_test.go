package prose

import (
	"strings"
	"testing"
)

// held builds a count table by hand. A case then states its two rates and
// leaves the corpus out.
func held(words int, grams map[string]int) *Counts {
	counts := &Counts{Grams: map[string]*Gram{}, Words: words}
	for key, count := range grams {
		counts.Grams[key] = &Gram{Skeleton: key, Count: count, Phrase: key, File: "a.go", Line: 1}
	}
	return counts
}

// TestCompareRate pins the comparison to rates. The corpora differ in size,
// and the same count over a tenth of the prose marks ten times the habit.
func TestCompareRate(t *testing.T) {
	found := Compare(
		held(1000, map[string]int{"is what": 10}),
		held(100000, map[string]int{"is what": 100}),
		Limits{Count: 1, Ratio: 1, Score: 0},
	)

	if len(found) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(found), found)
	}
	if got := found[0].Ratio; got != 10 {
		t.Errorf("ratio is %.1f, want 10", got)
	}
}

// TestCompareAbsent names a shape the baseline never used in words. A figure
// standing for a division by zero reads as a measurement.
func TestCompareAbsent(t *testing.T) {
	found := Compare(
		held(1000, map[string]int{"beside": 10}),
		held(100000, nil),
		Limits{Count: 1, Ratio: 1, Score: 0},
	)

	if len(found) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(found), found)
	}
	if !found[0].Absent || found[0].Ratio != 0 {
		t.Errorf("a shape the baseline never used reads as %+v", found[0])
	}
	if got := shapeMessage(found[0]); !strings.Contains(got, "never writes it") {
		t.Errorf("the message reads %q, want it to say the baseline never writes the shape", got)
	}
}

// TestCompareRanksEvidence pins the ranking off the ratio. Three uses in a
// thousand words against one in a hundred thousand reads as three hundred
// times the rate. The small sample accounts for most of that. A hundred
// uses against a thousand reads as ten times the rate and settles it. The
// second ranks first.
func TestCompareRanksEvidence(t *testing.T) {
	found := Compare(
		held(1000, map[string]int{"thin": 3, "thick": 100}),
		held(100000, map[string]int{"thin": 1, "thick": 1000}),
		Limits{Count: 1, Ratio: 1, Score: 0},
	)

	if len(found) != 2 {
		t.Fatalf("got %d findings, want 2: %+v", len(found), found)
	}
	if found[0].Skeleton != "thick" {
		t.Errorf("ranked %q first, want \"thick\": %+v", found[0].Skeleton, found)
	}
	if found[0].Ratio >= found[1].Ratio {
		t.Error("the case is only interesting when the first finding has the lower ratio")
	}
}

// TestCompareLimits raises one floor at a time, leaving the other two loose,
// and checks that every finding Compare returned clears the raised one.
func TestCompareLimits(t *testing.T) {
	tree := held(1000, map[string]int{"rare": 2, "common": 50})
	baseline := held(100000, map[string]int{"rare": 1, "common": 4000})

	for name, limits := range map[string]Limits{
		"count": {Count: 10, Ratio: 1, Score: 0},
		"ratio": {Count: 1, Ratio: 100, Score: 0},
		"score": {Count: 1, Ratio: 1, Score: 1000},
	} {
		t.Run(name, func(t *testing.T) {
			for _, one := range Compare(tree, baseline, limits) {
				switch name {
				case "count":
					if one.Count < limits.Count {
						t.Errorf("%q cleared a count limit of %d with %d", one.Skeleton, limits.Count, one.Count)
					}
				case "ratio":
					if !one.Absent && one.Ratio < limits.Ratio {
						t.Errorf("%q cleared a ratio limit of %.0f with %.1f", one.Skeleton, limits.Ratio, one.Ratio)
					}
				case "score":
					if one.Score < limits.Score {
						t.Errorf("%q cleared a score limit of %.0f with %.2f", one.Skeleton, limits.Score, one.Score)
					}
				}
			}
		})
	}
}

// TestLikelihoodZero pins the limit where one side holds none. The measure
// answers a figure and never divides by zero.
func TestLikelihoodZero(t *testing.T) {
	if got := likelihood(10, 0, 1000, 100000); got <= 0 {
		t.Errorf("a shape the baseline never used scores %.2f, want more than nothing", got)
	}
	if got := likelihood(0, 0, 1000, 100000); got != 0 {
		t.Errorf("a shape neither corpus holds scores %.2f, want 0", got)
	}
	if got := likelihood(10, 1, 0, 0); got != 0 {
		t.Errorf("a measure over no prose scores %.2f, want 0", got)
	}
}

// TestLikelihoodEven answers zero where both corpora use a shape at one
// rate. Prose reading like its baseline reports no finding.
func TestLikelihoodEven(t *testing.T) {
	if got := likelihood(10, 1000, 1000, 100000); got > 0.000001 {
		t.Errorf("two equal rates score %.6f, want 0", got)
	}
}
