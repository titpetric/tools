package pairing

import (
	"fmt"
	"iter"
	"sort"
	"strconv"

	"github.com/titpetric/tools/splint/model"
)

// Result is one unpaired file, in the terms this linter thinks in.
type Result struct {
	// Rule is which of the linter's rules the finding is under.
	Rule string

	// Position is the file the finding is about, which has no line: nothing in
	// the file is wrong, the file is the finding.
	Position model.Position

	// Message names the test that would have paired with it.
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
	var files, paired, standaloneFiles, standaloneTests int

	for _, path := range r.order {
		metric := r.packages[path]
		files += metric.Files
		paired += metric.Paired
		standaloneFiles += metric.StandaloneFiles
		standaloneTests += metric.StandaloneTests
		rows = append(rows, []string{
			path,
			strconv.Itoa(metric.Files),
			strconv.Itoa(metric.Tests),
			strconv.Itoa(metric.Paired),
			strconv.Itoa(metric.StandaloneFiles),
			strconv.Itoa(metric.StandaloneTests),
		})
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })

	return []model.Statistics{model.NewStatistics(
		[]string{"Package", "Files", "Tests", "Paired", "Standalone files", "Standalone tests"},
		rows,
		model.HeaderText("Files and the tests named after them, by package."),
		model.FooterText(fmt.Sprintf("%d of %d files have a test beside them across %s, leaving %s and %s standing alone.",
			paired, files, plural(len(r.order), "package"), plural(standaloneFiles, "file"), plural(standaloneTests, "test"))),
	)}
}

// plural counts a thing in a sentence, so the one line of summary reads as a
// sentence when the count happens to be one.
func plural(count int, word string) string {
	if count == 1 {
		return "1 " + word
	}
	return strconv.Itoa(count) + " " + word + "s"
}

// count records what one package held, and returns what to record against.
//
// The key is the import path of the half a consumer imports, so the metrics of
// a package and of its test package land on the one entry rather than on an
// "x" and an "x_test" a reader has to add up.
func (r *Results) count(pkg model.Package, files, tests int) *Metric {
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
	metric.Files += files
	metric.Tests += tests
	return metric
}

// add records one finding against the package it was found in. Every finding
// is a file standing alone, so the counter is the findings counted.
func (r *Results) add(metric *Metric, result Result) {
	r.findings = append(r.findings, result)
	metric.StandaloneFiles++
}
