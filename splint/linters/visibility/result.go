package visibility

import (
	"fmt"
	"iter"
	"sort"
	"strconv"

	"github.com/titpetric/tools/splint/model"
)

// Results is what the linter counted. It holds no findings.
//
// The report interface carries findings and numbers together, and a check is
// free to bring only the numbers. This one does: Len is zero and All yields
// nothing, and everything the linter has to say is in the table.
type Results struct {
	packages map[string]*Metric
	order    []string
}

// Linter names the linter the report came from.
func (r Results) Linter() string {
	return Name
}

// Len is how many findings there are, which is none: the counts are reported
// and not judged.
func (r Results) Len() int {
	return 0
}

// All yields every finding, of which there are none.
func (r Results) All() iter.Seq[model.Issue] {
	return func(yield func(model.Issue) bool) {}
}

// Metrics is what the linter counted, per package.
func (r Results) Metrics() model.LintMetrics {
	metrics := model.LintMetrics{}
	for path, metric := range r.packages {
		metrics.AddPackage(path, *metric)
	}
	return metrics
}

// Statistics is the count as one table, by package path, so a reader finds a
// package where they expect it rather than where its numbers put it.
func (r Results) Statistics() []model.Statistics {
	rows := make([][]string, 0, len(r.order))
	var exported, internal int

	for _, path := range r.order {
		metric := r.packages[path]
		exported += metric.Exported()
		internal += metric.Internal()
		rows = append(rows, []string{
			path,
			pair(metric.ExportedTypes, metric.InternalTypes),
			pair(metric.ExportedFuncs, metric.InternalFuncs),
			fmt.Sprintf("%.1f%%", metric.Ratio()),
		})
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })

	return []model.Statistics{model.NewStatistics(
		[]string{"Package", "Types", "Funcs", "Ratio"},
		rows,
		model.HeaderText("What every package exports against what it keeps. Types and funcs read exported / internal, and the ratio is internal code over package code."),
		model.FooterText(fmt.Sprintf("%d packages, %d symbols exported and %d kept, %s of the code behind the internal half.",
			len(r.order), exported, internal, share(r.packages))),
	)}
}

// count records what one package held, and returns what to record against.
func (r *Results) count(def *model.Definition) *Metric {
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
	return metric
}

// pair renders the two halves of a count the way the table reads them.
func pair(exported, internal int) string {
	return strconv.Itoa(exported) + " / " + strconv.Itoa(internal)
}

// share is the internal ratio of everything measured together, which is not
// the average of the per package ratios: a large package weighs what its code
// weighs.
func share(packages map[string]*Metric) string {
	var internal, lines int
	for _, metric := range packages {
		internal += metric.InternalLines
		lines += metric.Lines
	}
	if lines == 0 {
		return "no code"
	}
	return fmt.Sprintf("%.1f%%", float64(internal)/float64(lines)*100)
}
