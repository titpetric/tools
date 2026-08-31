package godoc

import (
	"fmt"
	"iter"
	"sort"
	"strconv"

	"github.com/titpetric/tools/splint/model"
)

// Result is one godoc finding, in the terms this linter thinks in.
type Result struct {
	// Rule is which of the godoc rules the finding is under.
	Rule string

	// Symbol is the declaration, receiver and name, and Kind is what it
	// declares.
	Symbol string
	Kind   model.DeclarationKind

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
		Attrs:    map[string]string{"kind": r.Kind.String()},
	}
}

// Results is what the linter found and what it counted.
//
// The findings keep their own shape and are turned into issues one at a time,
// rather than the whole slice being copied into a common one.
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

// Statistics is the count as one table, worst documented first: a package a
// reader should look at is a package near the top.
func (r Results) Statistics() []model.Statistics {
	rows := make([][]string, 0, len(r.order))
	var exported, documented int

	for _, path := range r.order {
		metric := r.packages[path]
		exported += metric.Exported
		documented += metric.Documented
		rows = append(rows, []string{
			path,
			strconv.Itoa(metric.Exported),
			strconv.Itoa(metric.Documented),
			percent(metric.Documented, metric.Exported),
			strconv.Itoa(metric.Missing),
			strconv.Itoa(metric.Format),
			strconv.Itoa(metric.Verbose),
		})
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })

	return []model.Statistics{model.NewStatistics(
		[]string{"Package", "Exported", "Documented", "Share", "Missing", "Format", "Verbose"},
		rows,
		model.HeaderText("Documentation of every exported symbol, by package."),
		model.FooterText(fmt.Sprintf("%d of %d exported symbols documented, %s, %d issues.",
			documented, exported, percent(documented, exported), len(r.findings))),
	)}
}

// count records what one package held.
func (r *Results) count(pkg model.Package, exported int) *Metric {
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
	metric.Exported += exported
	return metric
}

// add records one finding against the package it was found in.
func (r *Results) add(metric *Metric, result Result) {
	r.findings = append(r.findings, result)

	switch result.Rule {
	case RuleMissing:
		metric.Missing++
	case RuleFormat:
		metric.Format++
	case RuleVerbose:
		metric.Verbose++
	}
}

// percent renders a share, and reads as nothing when there was nothing to
// take a share of.
func percent(part, whole int) string {
	if whole == 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", float64(part)/float64(whole)*100)
}
