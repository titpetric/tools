package integration

import (
	"fmt"
	"iter"
	"sort"
	"strconv"

	"github.com/titpetric/tools/splint/model"
)

// Result is one finding, in the terms this linter thinks in.
type Result struct {
	// Rule is which of the linter's rules the finding is under, and Severity
	// how much it weighs.
	Rule     string
	Severity model.Severity

	// Symbol is the test or the file the finding is about, and Position its
	// location.
	Symbol   string
	Position model.Position

	// Reason is how a test crosses the boundary, for the boundary rule.
	Reason string

	// Message says what is wrong and what to do about it.
	Message string
}

// Issue renders the finding as the framework reads it.
func (r Result) Issue() model.Issue {
	issue := model.Issue{
		Linter:   Name,
		Rule:     r.Rule,
		Severity: r.Severity,
		Position: r.Position,
		Symbol:   r.Symbol,
		Message:  r.Message,
	}
	if r.Reason != "" {
		issue.Attrs = map[string]string{"reason": r.Reason}
	}
	return issue
}

// Results is what the linter found and what it counted.
type Results struct {
	findings []Result
	packages map[string]*Metric
	order    []string
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

// Metrics is what the linter counted, per package.
func (r Results) Metrics() model.LintMetrics {
	metrics := model.LintMetrics{}
	for path, metric := range r.packages {
		metrics.AddPackage(path, *metric)
	}
	return metrics
}

// Statistics is the count as one table, of the packages holding tests.
func (r Results) Statistics() []model.Statistics {
	rows := make([][]string, 0, len(r.order))
	var tests, crossing, integration int

	for _, path := range r.order {
		metric := r.packages[path]
		if metric.Tests == 0 && metric.Integration == 0 {
			continue
		}
		tests += metric.Tests
		crossing += metric.Crossing
		integration += metric.Integration
		rows = append(rows, []string{
			path,
			strconv.Itoa(metric.Tests),
			strconv.Itoa(metric.Crossing),
			strconv.Itoa(metric.Integration),
		})
	}
	if len(rows) == 0 {
		return nil
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })

	return []model.Statistics{model.NewStatistics(
		[]string{"Package", "Tests", "Crossing", "Integration files"},
		rows,
		model.HeaderText("Tests a plain go test runs, the ones crossing a process boundary, and the files behind the tag, by package."),
		model.FooterText(fmt.Sprintf("%d of %d tests cross a process boundary outside the tag, %d files behind it.",
			crossing, tests, integration)),
	)}
}

// count returns the metric of a package, recording it on first sight.
func (r *Results) count(pkg model.Package) *Metric {
	if r.packages == nil {
		r.packages = map[string]*Metric{}
	}

	path := pkg.Path
	metric, known := r.packages[path]
	if !known {
		metric = &Metric{}
		r.packages[path] = metric
		r.order = append(r.order, path)
	}
	return metric
}

// add records one finding.
func (r *Results) add(result Result) {
	r.findings = append(r.findings, result)
}
