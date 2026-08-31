package wraphandler

import (
	"fmt"
	"iter"
	"sort"
	"strconv"

	"github.com/titpetric/tools/splint/model"
)

// Result is one finding, in the terms this linter thinks in.
type Result struct {
	// Rule is which of the linter's rules the finding is under.
	Rule string

	// Symbol is what the finding is about, and Position where it is.
	Symbol   string
	Position model.Position

	// Message says what is wrong.
	Message string
}

// Issue renders the finding as the framework reads it.
func (r Result) Issue() model.Issue {
	return model.Issue{
		Linter:   Name,
		Rule:     r.Rule,
		Severity: model.SeverityWarn,
		Position: r.Position,
		Symbol:   r.Symbol,
		Message:  r.Message,
	}
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

// All yields every finding as an issue.
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

// Statistics is the count as one table. Only a package that serves something
// is in it: a package with no handlers has no share to report, and a row of
// zeroes for every other package in the tree buries the ones that do.
func (r Results) Statistics() []model.Statistics {
	rows := make([][]string, 0, len(r.order))
	var handlers, wrapped int

	for _, path := range r.order {
		metric := r.packages[path]
		handlers += metric.Handlers
		wrapped += metric.Wrapped
		rows = append(rows, []string{
			path,
			strconv.Itoa(metric.Handlers),
			strconv.Itoa(metric.Wrapped),
			percent(metric.Wrapped, metric.Handlers),
		})
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })

	return []model.Statistics{model.NewStatistics(
		[]string{"Package", "Handlers", "Wrapped", "Share"},
		rows,
		model.HeaderText("Exported HTTP handlers and the functions behind them, by package."),
		model.FooterText(fmt.Sprintf("%d of %d exported handlers wrapped, %s, %d testable only through a server.",
			wrapped, handlers, percent(wrapped, handlers), len(r.findings))),
	)}
}

// count records how many handlers one package held, and returns what to record
// against.
func (r *Results) count(pkg model.Package, handlers int) *Metric {
	if r.packages == nil {
		r.packages = map[string]*Metric{}
	}

	path := pkg.ImportPath
	if path == "" {
		path = pkg.Path
	}

	metric, known := r.packages[path]
	if !known {
		metric = &Metric{}
		r.packages[path] = metric
		r.order = append(r.order, path)
	}
	metric.Handlers += handlers
	return metric
}

// add records one finding.
func (r *Results) add(result Result) {
	r.findings = append(r.findings, result)
}

// percent renders a share, and reads as nothing when there was nothing to take
// a share of.
func percent(part, whole int) string {
	if whole == 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", float64(part)/float64(whole)*100)
}
