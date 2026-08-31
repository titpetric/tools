package model

import (
	"context"
	"iter"
)

// Linter reads a parsed document and reports what it found.
//
// A linter never touches disk and never parses anything: everything it needs
// is in the document, which is what lets the same rule run over a tree the ast
// parser read and one the simple parser read, and over a document loaded back
// from a file.
type Linter interface {
	// Name is how the linter is selected and how its issues are labelled.
	Name() string

	// Lint reports what the linter found. A linter that found nothing returns
	// an empty report and no error; an error is the linter failing, not the
	// code failing.
	Lint(ctx context.Context, root *DocumentRoot) (LintReport, error)
}

// LintReport is what one linter found and what it measured.
//
// It is an interface rather than a slice so a linter keeps its own result type
// and the framework ranges over it in place: a rule that carries twenty fields
// per finding is walked without copying any of them into a common struct, and
// only the Issue view of each is materialised.
//
// Findings and numbers come back together because they are found together. A
// check that counts what it looked at has the count in hand by the time it
// knows what to report, and asking for it twice would be running it twice.
type LintReport interface {
	// Linter names the linter the report came from.
	Linter() string

	// Len is how many issues the report holds, answered without walking it.
	Len() int

	// All yields every issue, in the order the linter found them.
	All() iter.Seq[Issue]

	// Metrics is what the linter measured, keyed by file or by package. A
	// linter that measures nothing returns the zero value.
	Metrics() LintMetrics

	// Statistics is how the metrics read, one table each. A linter that
	// measures nothing returns none.
	Statistics() []Statistics
}

// Issues collects a report into a slice, for a caller that needs one.
func Issues(report LintReport) []Issue {
	if report == nil {
		return nil
	}
	issues := make([]Issue, 0, report.Len())
	for issue := range report.All() {
		issues = append(issues, issue)
	}
	return issues
}
