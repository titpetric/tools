package deprecated

import (
	"iter"

	"github.com/titpetric/tools/splint/model"
)

// Result is one use of a deprecated symbol, at the line it is written on.
type Result struct {
	// Symbol is the deprecated symbol, and Position is the use.
	Symbol   string
	Position model.Position

	// Message names the symbol and the reason its doc gave.
	Message string
}

// Issue renders the finding as the framework reads it.
func (r Result) Issue() model.Issue {
	return model.Issue{
		Linter:   Name,
		Rule:     RuleUse,
		Severity: model.SeverityError,
		Position: r.Position,
		Symbol:   r.Symbol,
		Message:  r.Message,
	}
}

// Results is what the linter found.
type Results struct {
	findings []Result
}

// Linter names the linter the report came from.
func (r Results) Linter() string {
	return Name
}

// Len is how many uses there are.
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

// Metrics is empty: the linter counts no numbers.
func (r Results) Metrics() model.LintMetrics {
	return model.LintMetrics{}
}

// Statistics is empty: the linter prints no table.
func (r Results) Statistics() []model.Statistics {
	return nil
}

// add records one use.
func (r *Results) add(result Result) {
	r.findings = append(r.findings, result)
}
