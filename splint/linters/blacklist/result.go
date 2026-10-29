package blacklist

import (
	"iter"

	"github.com/titpetric/tools/splint/model"
)

// Result is one finding, in the terms this linter thinks in.
type Result struct {
	// Rule is which of the linter's rules the finding is under.
	Rule string

	// Symbol is the import path the finding is about, and Position is the
	// file that writes it.
	Symbol   string
	Position model.Position

	// Replacement is the path the tree should import instead, and is empty
	// when the linter knows none.
	Replacement string

	// Message says what is wrong.
	Message string
}

// Issue renders the finding as the framework reads it.
func (r Result) Issue() model.Issue {
	issue := model.Issue{
		Linter:   Name,
		Rule:     r.Rule,
		Severity: model.SeverityWarn,
		Position: r.Position,
		Symbol:   r.Symbol,
		Message:  r.Message,
	}
	if r.Replacement != "" {
		issue.Attrs = map[string]string{"replacement": r.Replacement}
	}
	return issue
}

// Results is what the linter found.
type Results struct {
	findings []Result
}

// Linter names the linter the report came from.
func (r Results) Linter() string {
	return Name
}

// Len is how many findings there are.
func (r Results) Len() int {
	return len(r.findings)
}

// All returns every finding as an issue.
func (r Results) All() iter.Seq[model.Issue] {
	return func(yield func(model.Issue) bool) {
		for _, result := range r.findings {
			if !yield(result.Issue()) {
				return
			}
		}
	}
}

// Metrics is what the linter measured, which is nothing: a finding is about a
// line of source, not about a number the tree earns.
func (r Results) Metrics() model.LintMetrics {
	return model.LintMetrics{}
}

// Statistics is the count as no table, for the same reason.
func (r Results) Statistics() []model.Statistics {
	return nil
}

// add records one finding.
func (r *Results) add(result Result) {
	r.findings = append(r.findings, result)
}
