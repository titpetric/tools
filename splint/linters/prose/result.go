package prose

import (
	"fmt"
	"iter"
	"strconv"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// Result is one prose finding.
type Result struct {
	// Rule is which of the prose rules the finding is under.
	Rule string

	// Position is where to go and look, and Message says what is there.
	Position model.Position
	Message  string
}

// Issue renders the finding as a warning: a tendency, never a defect.
func (r Result) Issue() model.Issue {
	return model.Issue{
		Linter:   Name,
		Rule:     r.Rule,
		Severity: model.SeverityWarn,
		Position: r.Position,
		Message:  r.Message,
	}
}

// Results is what the linter found and what it measured.
type Results struct {
	findings []Result

	// baseline names the tree the measure ran against, and tree and corpus
	// are the two densities.
	baseline string
	tree     Density
	corpus   Density

	// dropped is how many shapes cleared the floors and missed the cap. A
	// report that truncates in silence reads as one that found everything.
	dropped int
}

// Linter names the linter the report came from.
func (r *Results) Linter() string {
	return Name
}

// Len is how many findings there are.
func (r *Results) Len() int {
	return len(r.findings)
}

// All returns every finding as an issue.
func (r *Results) All() iter.Seq[model.Issue] {
	return func(yield func(model.Issue) bool) {
		for _, result := range r.findings {
			if !yield(result.Issue()) {
				return
			}
		}
	}
}

// Metrics is what the linter counted. The measure covers the tree whole and
// goes under one key.
func (r *Results) Metrics() model.LintMetrics {
	metrics := model.LintMetrics{}
	if r.tree.Blocks == 0 {
		return metrics
	}

	metrics.AddPackage(".", Metric{
		Blocks:        r.tree.Blocks,
		Words:         r.tree.Words,
		BlockWords:    r.tree.BlockWords,
		SentenceWords: r.tree.SentenceWords,
		P90Words:      r.tree.P90Words,
		Subordinators: r.tree.Subordinators,
		Stacked:       r.tree.Stacked,
		Shapes:        len(r.findings),
	})
	return metrics
}

// Statistics is the measure as one table, the tree beside its baseline.
func (r *Results) Statistics() []model.Statistics {
	if r.tree.Blocks == 0 {
		return nil
	}

	rows := [][]string{
		{"Comment blocks", strconv.Itoa(r.tree.Blocks), strconv.Itoa(r.corpus.Blocks), ""},
		{"Words", strconv.Itoa(r.tree.Words), strconv.Itoa(r.corpus.Words), ""},
		{"Median words per block", strconv.Itoa(r.tree.BlockWords), strconv.Itoa(r.corpus.BlockWords), over(float64(r.tree.BlockWords), float64(r.corpus.BlockWords))},
		{"Median words per sentence", strconv.Itoa(r.tree.SentenceWords), strconv.Itoa(r.corpus.SentenceWords), over(float64(r.tree.SentenceWords), float64(r.corpus.SentenceWords))},
		{"P90 words per sentence", strconv.Itoa(r.tree.P90Words), strconv.Itoa(r.corpus.P90Words), over(float64(r.tree.P90Words), float64(r.corpus.P90Words))},
		{"Subordinators per 1000 words", rate(r.tree.Subordinators), rate(r.corpus.Subordinators), over(r.tree.Subordinators, r.corpus.Subordinators)},
		{"Commas per 1000 words", rate(r.tree.Commas), rate(r.corpus.Commas), over(r.tree.Commas, r.corpus.Commas)},
		{"Blocks stacking subordinators", rate(r.tree.Stacked) + "%", rate(r.corpus.Stacked) + "%", over(r.tree.Stacked, r.corpus.Stacked)},
	}

	footer := fmt.Sprintf("%s above the baseline. A ratio of 1.0 reads like the baseline's prose.",
		plural(len(r.findings), "finding", "findings"))
	if r.dropped > 0 {
		footer += fmt.Sprintf(" %d more cleared the floors and did not fit the cap.", r.dropped)
	}

	return []model.Statistics{model.NewStatistics(
		[]string{"Measure", "Tree", "Baseline", "Over"},
		rows,
		model.HeaderText("How the prose reads, against "+r.baseline+"."),
		model.FooterText(footer),
	)}
}

// add records one finding.
func (r *Results) add(result Result) {
	r.findings = append(r.findings, result)
}

// shapeMessage says what a shape is and how far over the baseline it runs.
//
// The phrase prints only where it differs from the shape. A two word shape is
// two function words most of the time. Printing "and a" twice in one sentence
// adds nothing to the first.
func shapeMessage(one Finding) string {
	margin := "the baseline never writes it"
	if !one.Absent {
		margin = fmt.Sprintf("%.0fx the baseline's rate", one.Ratio)
	}

	shape := strconv.Quote(one.Skeleton)
	if one.Phrase != one.Skeleton {
		shape += fmt.Sprintf(" (%q)", one.Phrase)
	}
	if strings.Contains(one.Skeleton, "X") {
		margin += ". X stands for a word of the subject matter"
	}

	return fmt.Sprintf("%s runs %s, %s.", shape, plural(one.Count, "time", "times"), margin)
}

// over is a ratio of two rates, and reads as nothing where there is none to
// take.
func over(part, whole float64) string {
	if whole == 0 || part == 0 {
		return ""
	}
	return fmt.Sprintf("%.1fx", part/whole)
}

// rate renders a per-thousand or per-hundred measure.
func rate(value float64) string {
	return fmt.Sprintf("%.2f", value)
}

// plural writes a count with the word for it.
func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}
