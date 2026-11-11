// Package prose reports the constructions a tree leans on that its baseline
// does not.
//
// A phrase list catches what somebody thought to name. This catches what
// nobody named: it masks the subject matter out of every sentence, counts
// what remains, and ranks whatever the tree uses more often per word than a
// corpus of writing worth trusting. The rules fall out of the measurement.
//
// It reads doc comments and markdown paragraphs together, because a sentence
// in a README and a sentence over a function answer to the same rule.
package prose

import (
	"context"
	"fmt"

	"github.com/titpetric/tools/splint/model"
)

// Name selects the linter and labels its issues.
const Name = "prose"

// The rules the linter reports under.
const (
	// RuleShape is one construction running above the baseline.
	RuleShape = "shape"

	// RuleDensity is one measure of sentence shape running above it: how far
	// a comment block runs, how far a sentence runs, how many clauses attach
	// a reason.
	RuleDensity = "density"

	// RulePlacement is a comment a configuration file writes below its top level.
	RulePlacement = "placement"
)

// Defaults is the floor a finding clears, and the shape size.
//
// The score is the evidence. Corpus work draws the line at 15.13: a gap
// scoring higher turns up by chance once in ten thousand comparisons. The
// ratio and the count are there to keep the report short, not to decide what
// is true.
var Defaults = Options{Size: 2, Count: 3, Ratio: 10, Score: 15.13, Top: 25}

// Options are what a run of the linter measures against.
type Options struct {
	// Baseline is the tree to compare against. Empty means the standard
	// library.
	Baseline string

	// Size is how many words a shape is. One finds a single word the
	// baseline avoids, two a phrase, three a clause.
	Size int

	// Count, Ratio and Score fix the floor a shape clears.
	Count int
	Ratio float64
	Score float64

	// Top caps the shapes reported, and zero reports every one.
	Top int
}

// Linter measures the prose of a document against a baseline.
type Linter struct {
	options Options
}

// New returns the linter at its defaults.
func New() *Linter {
	return &Linter{options: Defaults}
}

// Configure takes the options a run names. A zero field keeps its default.
func (l *Linter) Configure(opts Options) {
	if opts.Baseline != "" {
		l.options.Baseline = opts.Baseline
	}
	if opts.Size > 0 {
		l.options.Size = opts.Size
	}
	if opts.Count > 0 {
		l.options.Count = opts.Count
	}
	if opts.Ratio > 0 {
		l.options.Ratio = opts.Ratio
	}
	if opts.Score > 0 {
		l.options.Score = opts.Score
	}
	if opts.Top > 0 {
		l.options.Top = opts.Top
	}
}

// Name selects the linter and labels its issues.
func (l *Linter) Name() string {
	return Name
}

// Lint reports the shapes and the densities running above the baseline.
//
// A tree with no prose reports no finding. A run holding no baseline reports
// none either. A machine with no Go toolchain is that run: the standard
// library is the baseline. The other linters run on regardless.
func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	// Placement is structural and answers to no baseline, which is why it is
	// measured ahead of the two returns below.
	placement := placementFindings(root.Prose)

	corpus := FromDocument(root)
	if len(corpus.Blocks) == 0 {
		return withPlacement(&Results{}, placement), nil
	}

	// A run with no baseline has no toolchain to read the standard library
	// from. The rule reports nothing, and never measures against nothing.
	path := l.options.Baseline
	if path == "" {
		return withPlacement(&Results{}, placement), nil
	}

	baseline, err := LoadBaseline(path, l.options.Size)
	if err != nil {
		return nil, fmt.Errorf("measuring the baseline at %s: %w", path, err)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// The lengths compare doc comments alone, since the baseline is doc
	// comments. The shapes read the markdown too. A shape's rate is per
	// word. A construction counts wherever it stands.
	results := &Results{
		baseline: path,
		tree:     Measure(corpus.Docs()),
		corpus:   baseline.Density,
	}

	found := Compare(Count(corpus, l.options.Size), baseline.Counts, Limits{
		Count: l.options.Count,
		Ratio: l.options.Ratio,
		Score: l.options.Score,
	})

	results.dropped = len(found)
	if l.options.Top > 0 && l.options.Top < len(found) {
		found = found[:l.options.Top]
	}
	results.dropped -= len(found)

	for _, one := range found {
		results.add(Result{
			Rule:     RuleShape,
			Position: model.Position{File: one.File, Line: one.Line},
			Message:  shapeMessage(one),
		})
	}

	for _, one := range densityFindings(results.tree, baseline.Density) {
		results.add(one)
	}

	return withPlacement(results, placement), nil
}

func withPlacement(results *Results, placement []Result) *Results {
	for _, one := range placement {
		results.add(one)
	}
	return results
}
