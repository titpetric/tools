package funcreturns

import (
	"fmt"
	"iter"
	"sort"
	"strconv"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// Result is one return order finding, in the terms this linter thinks in.
type Result struct {
	// Rule is which of the return rules the finding is under.
	Rule string

	// Symbol is the function, receiver and name. Returns are the types it
	// returns and Expected the order they should read in.
	Symbol   string
	Returns  []string
	Expected []string

	// Position is where it is, and Message says what is wrong.
	Position model.Position
	Message  string
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
		Attrs: map[string]string{
			"returns":  strings.Join(r.Returns, ", "),
			"expected": strings.Join(r.Expected, ", "),
		},
	}
}

// HeaderLine is what the statistics table says it is.
const HeaderLine = "Return order of every func returning more than one value, by package."

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

// Statistics is the count as one table.
func (r Results) Statistics() []model.Statistics {
	rows := make([][]string, 0, len(r.order))
	var funcs, considered, passing int

	for _, path := range r.order {
		metric := r.packages[path]
		funcs += metric.Funcs
		considered += metric.Considered
		passing += metric.Passing
		rows = append(rows, []string{
			path,
			strconv.Itoa(metric.Funcs),
			strconv.Itoa(metric.Considered),
			strconv.Itoa(metric.Passing),
			percent(metric.Passing, metric.Considered),
		})
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })

	return []model.Statistics{model.NewStatistics(
		[]string{"Package", "Funcs", "Considered", "Passing", "Share"},
		rows,
		model.HeaderText(HeaderLine),
		model.FooterText(fmt.Sprintf("%d of %d funcs considered, %d passing, %s, %d issues.",
			considered, funcs, passing, percent(passing, considered), len(r.findings))),
	)}
}

// count records what one package held, and returns what to record against.
func (r *Results) count(pkg model.Package, funcs, considered int) *Metric {
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
	metric.Funcs += funcs
	metric.Considered += considered
	return metric
}

// add records one finding against the package it was found in.
func (r *Results) add(metric *Metric, result Result) {
	r.findings = append(r.findings, result)
}

// pass records a func that was considered and had nothing wrong with it.
func (r *Results) pass(metric *Metric, count int) {
	metric.Passing += count
}

// percent renders a share, and reads as nothing when there was nothing to
// take a share of.
func percent(part, whole int) string {
	if whole == 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", float64(part)/float64(whole)*100)
}
