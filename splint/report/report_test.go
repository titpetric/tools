package report_test

import (
	"iter"
	"testing"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/report"
)

// results is a linter report standing in for a real one, which is all the
// framework ever sees of a linter.
type results struct {
	name   string
	issues []model.Issue
}

func (r results) Linter() string                 { return r.name }
func (r results) Len() int                       { return len(r.issues) }
func (r results) Metrics() model.LintMetrics     { return model.LintMetrics{} }
func (r results) Statistics() []model.Statistics { return nil }
func (r results) All() iter.Seq[model.Issue] {
	return func(yield func(model.Issue) bool) {
		for _, issue := range r.issues {
			if !yield(issue) {
				return
			}
		}
	}
}

func sample() *report.Report {
	return report.New(
		results{name: "godoc", issues: []model.Issue{
			{
				Linter: "godoc", Rule: "missing", Severity: model.SeverityWarn,
				Position: model.Position{Package: "view", File: "frontend/view/page.go", Line: 42},
				Symbol:   "Page", Message: "exported symbol lacks a godoc comment",
			},
			{
				Linter: "godoc", Rule: "format", Severity: model.SeverityError,
				Position: model.Position{Package: "model", File: "model/trace.go", Line: 7},
				Symbol:   "Trace", Message: "godoc should end in punctuation",
			},
		}},
		results{name: "imports"},
	)
}

// TestNewSortsByPosition covers the order a report reads in: down a tree,
// rather than jumping between linters.
func TestNewSortsByPosition(t *testing.T) {
	got := sample()

	if got.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", got.Len())
	}
	if got.Issues[0].Position.File != "frontend/view/page.go" {
		t.Errorf("first issue is %q", got.Issues[0].Position.File)
	}
	if got.Worst() != model.SeverityError {
		t.Errorf("Worst() = %v, want error", got.Worst())
	}
	// A linter that found nothing is still named: the report says what ran.
	if len(got.Linters) != 2 {
		t.Errorf("Linters = %v", got.Linters)
	}
	if counts := got.Counts(); counts["godoc"] != 2 || counts["imports"] != 0 {
		t.Errorf("Counts() = %v", counts)
	}
}
