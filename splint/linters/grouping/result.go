package grouping

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

	// Symbol is what the finding is about, and Position its location.
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

// Statistics is one row per package: its exported symbols, and how many sit in a file named for them.
func (r Results) Statistics() []model.Statistics {
	rows := make([][]string, 0, len(r.order))
	var symbols, passing, contained int

	for _, path := range r.order {
		metric := r.packages[path]
		symbols += metric.Symbols
		passing += metric.Passing
		contained += metric.SelfContained
		rows = append(rows, []string{
			path,
			strconv.Itoa(metric.Symbols),
			strconv.Itoa(metric.Passing),
			strconv.Itoa(metric.Violations),
			strconv.Itoa(metric.SelfContained),
			percent(metric.Passing, metric.Symbols),
		})
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })

	return []model.Statistics{model.NewStatistics(
		[]string{"Package", "Symbols", "Passing", "Violations", "Own file", "Share"},
		rows,
		model.HeaderText("Exported symbols and the files they are named for, by package. Own file is the symbols left alone for sitting in a file that compiles by itself."),
		model.FooterText(fmt.Sprintf("%d of %d exported symbols in a file named for them, %s, %d elsewhere, %d left to a file of their own.",
			passing, symbols, percent(passing, symbols), len(r.findings), contained)),
	)}
}

// count records what one package held, and returns what to record against.
func (r *Results) count(pkg model.Package, symbols, contained int) *Metric {
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

	metric.Symbols += symbols
	metric.Passing += symbols
	metric.SelfContained += contained
	return metric
}

// add records one finding against the package that holds it. The linter
// counts a symbol as passing when it reads the symbol and moves it across on report,
// because the count comes before the check runs and the finding after it.
func (r *Results) add(metric *Metric, result Result) {
	r.findings = append(r.findings, result)
	metric.Passing--
	metric.Violations++
}

// percent renders a share, and reads as nothing when the whole is zero.
func percent(part, whole int) string {
	if whole == 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", float64(part)/float64(whole)*100)
}
