package imports

import (
	"fmt"
	"iter"
	"sort"
	"strconv"

	"github.com/titpetric/tools/splint/model"
)

// Result is one import collision, in the terms this linter thinks in.
type Result struct {
	// Rule is which of the import rules the finding is under.
	Rule string

	// Position is the package the collision is in, which has no line: the
	// collision is between two files and belongs to neither.
	Position model.Position

	// Message names the two import paths that collided.
	Message string
}

// Issue renders the finding as the framework reads it.
func (r Result) Issue() model.Issue {
	return model.Issue{
		Linter:   Name,
		Rule:     r.Rule,
		Severity: model.SeverityWarn,
		Position: r.Position,
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

// Statistics is the count as one table.
func (r Results) Statistics() []model.Statistics {
	rows := make([][]string, 0, len(r.order))
	var files, names, collisions int

	for _, path := range r.order {
		metric := r.packages[path]
		files += metric.Files
		names += metric.Names
		collisions += metric.Collisions
		rows = append(rows, []string{
			path,
			strconv.Itoa(metric.Files),
			strconv.Itoa(metric.Names),
			strconv.Itoa(metric.Collisions),
		})
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })

	return []model.Statistics{model.NewStatistics(
		[]string{"Package", "Files", "Import names", "Collisions"},
		rows,
		model.HeaderText("The short names a package reaches its imports by."),
		model.FooterText(fmt.Sprintf("%d import names across %d files in %d packages, %d meaning two things.",
			names, files, len(r.order), collisions)),
	)}
}

// count records what one package held, and returns what to record against.
func (r *Results) count(def *model.Definition, names int) *Metric {
	if r.packages == nil {
		r.packages = map[string]*Metric{}
	}

	path := def.Package.ImportPath
	if path == "" {
		path = def.Package.Path
	}

	metric, known := r.packages[path]
	if !known {
		metric = &Metric{}
		r.packages[path] = metric
		r.order = append(r.order, path)
	}
	metric.Files += len(def.Imports)
	metric.Names += names
	return metric
}

// add records one finding against the package it was found in.
func (r *Results) add(metric *Metric, result Result) {
	r.findings = append(r.findings, result)
	metric.Collisions++
}
