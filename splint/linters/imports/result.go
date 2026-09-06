package imports

import (
	"fmt"
	"iter"
	"sort"
	"strconv"

	"github.com/titpetric/tools/splint/model"
)

// Result is one finding, in the terms this linter thinks in.
type Result struct {
	// Rule is which of the import rules the finding is under.
	Rule string

	// Severity is how much it means. A rule about what a file writes is an
	// error, because the file is not what the tree says a file is; a rule
	// about how far something has spread is a warning, because it is a
	// reading rather than a defect.
	Severity model.Severity

	// Position is where it is.
	Position model.Position

	// Symbol is the import or the name the finding is about, and is empty for
	// a finding about a block.
	Symbol string

	// Message is the finding in a sentence.
	Message string

	// Fixable reports a finding splint fix clears.
	Fixable bool
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
	if r.Fixable {
		issue.Attrs = map[string]string{FixAttr: FixCommand}
	}
	return issue
}

// Results is what the linter found and what it counted.
type Results struct {
	findings []Result
	packages map[string]*Metric
	order    []string

	// spread is how far every import path of the tree has reached, and files
	// how many files it was counted over.
	spread []*Spread
	files  int
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

// Statistics is the count as two tables: what each package's imports say, and
// how far each dependency has reached.
func (r Results) Statistics() []model.Statistics {
	return []model.Statistics{r.perPackage(), r.perDependency()}
}

// perPackage is what the import rules read in every package.
func (r Results) perPackage() model.Statistics {
	rows := make([][]string, 0, len(r.order))
	var files, names, collisions, considered, formatted, unused, unresolved int

	for _, path := range r.order {
		metric := r.packages[path]
		files += metric.Files
		names += metric.Names
		collisions += metric.Collisions
		considered += metric.Considered
		formatted += metric.Formatted
		unused += metric.Unused
		unresolved += metric.Unresolved

		rows = append(rows, []string{
			path,
			strconv.Itoa(metric.Files),
			strconv.Itoa(metric.Names),
			strconv.Itoa(metric.Collisions),
			strconv.Itoa(metric.Formatted) + "/" + strconv.Itoa(metric.Considered),
			strconv.Itoa(metric.Unused),
			strconv.Itoa(metric.Unresolved),
		})
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })

	footer := fmt.Sprintf("%d import names across %d files in %d packages, %d meaning two things. %d of %d files hold the block they should, %d imports are unreached and %d names do not resolve.",
		names, files, len(r.order), collisions, formatted, considered, unused, unresolved)

	return model.NewStatistics(
		[]string{"Package", "Files", "Import names", "Collisions", "Formatted", "Unused", "Unresolved"},
		rows,
		model.HeaderText("The short names a package reaches its imports by, and whether its files write them the way the tree does."),
		model.FooterText(footer),
	)
}

// perDependency is how far every import path has reached into the tree.
//
// One file is the clean reading and is what the column is read against. A
// package of the tree and a package of the standard library are listed and
// never judged: how far either reaches is a fact about the structure rather
// than about a dependency taken on.
func (r Results) perDependency() model.Statistics {
	rows := make([][]string, 0, len(r.spread))
	external, spread := 0, 0

	for _, entry := range r.spread {
		if !entry.External() {
			continue
		}
		external++
		if entry.Files > 1 {
			spread++
		}

		rows = append(rows, []string{
			entry.Path,
			strconv.Itoa(entry.Files),
			strconv.Itoa(entry.Packages),
			strconv.Itoa(entry.Worst),
			entry.WorstPackage,
		})
	}

	return model.NewStatistics(
		[]string{"Dependency", "Files", "Packages", "Most in one", "Which"},
		rows,
		model.HeaderText("How far each external dependency has reached. One file is one place to change when it goes."),
		model.FooterText(fmt.Sprintf("%d external dependencies over %d files, %d of them reaching more than one file.", external, r.files, spread)),
	)
}

// count records what one package held, and returns what to record against.
func (r *Results) count(def *model.Definition, names int) *Metric {
	path := def.Package.ImportPath
	if path == "" {
		path = def.Package.Path
	}

	metric := r.packageMetric(path)
	metric.Files += len(def.Imports)
	metric.Names += names

	return metric
}

// packageMetric is what a package is counted against, added when the package
// has not been counted yet.
func (r *Results) packageMetric(path string) *Metric {
	if r.packages == nil {
		r.packages = map[string]*Metric{}
	}
	if path == "" {
		path = "."
	}

	metric, known := r.packages[path]
	if !known {
		metric = &Metric{}
		r.packages[path] = metric
		r.order = append(r.order, path)
	}

	return metric
}

// add records one finding against the package it was found in.
func (r *Results) add(metric *Metric, result Result) {
	r.findings = append(r.findings, result)
	if result.Rule == RuleCollision {
		metric.Collisions++
	}
}
