package selfcontained

import (
	"fmt"
	"iter"
	"sort"
	"strconv"

	"github.com/titpetric/tools/splint/model"
)

// coupledFiles is how many of the most coupled files the second table names.
// It is a list to act on rather than an inventory.
const coupledFiles = 10

// Results is what the linter measured. It finds nothing, which the framework
// allows: an empty report and the tables.
type Results struct {
	// files is what each file measured, keyed by the path a position names,
	// and order is the order they were measured in.
	files map[string]*Metric
	order []string

	// packages are the directories measured, in document order, each holding
	// the paths of its own files.
	packages []*packageMetric
	byPath   map[string]*packageMetric
}

// packageMetric is one package directory and the files it is made of.
type packageMetric struct {
	importPath string
	files      []string
}

// pkg returns the package a measurement belongs to, adding it on first sight.
func (r *Results) pkg(importPath string) *packageMetric {
	if r.byPath == nil {
		r.byPath = map[string]*packageMetric{}
	}
	if known, ok := r.byPath[importPath]; ok {
		return known
	}

	one := &packageMetric{importPath: importPath}
	r.byPath[importPath] = one
	r.packages = append(r.packages, one)
	return one
}

// file returns what a file measured, adding it on first sight.
//
// A directory holding an external test package is two definitions of one path,
// so a file is recorded once however many definitions name it.
func (r *Results) file(pkg *packageMetric, at model.Position, file model.File) *Metric {
	path := at.Ref()

	if r.files == nil {
		r.files = map[string]*Metric{}
	}
	if known, ok := r.files[path]; ok {
		return known
	}

	metric := &Metric{Test: file.Test}
	r.files[path] = metric
	r.order = append(r.order, path)
	pkg.files = append(pkg.files, path)

	return metric
}

// Linter names the linter the report came from.
func (r Results) Linter() string {
	return Name
}

// Len is how many findings there are, which is none.
func (r Results) Len() int {
	return 0
}

// All yields nothing: the linter measures and does not judge.
func (r Results) All() iter.Seq[model.Issue] {
	return func(yield func(model.Issue) bool) {}
}

// Metrics is what the linter measured, per file.
func (r Results) Metrics() model.LintMetrics {
	metrics := model.LintMetrics{}
	for path, metric := range r.files {
		metrics.AddFile(path, *metric)
	}
	return metrics
}

// Statistics is the scope of every package, and then the files that reach
// furthest outside themselves.
func (r Results) Statistics() []model.Statistics {
	if len(r.order) == 0 {
		return nil
	}
	return []model.Statistics{r.byPackage(), r.mostCoupled()}
}

// byPackage is one row per package directory: what it declares, how much of it
// stands on its own, and the share that does not.
func (r Results) byPackage() model.Statistics {
	rows := make([][]string, 0, len(r.packages))
	var all, tests Metric
	var files int

	for _, pkg := range r.packages {
		var code, test Metric
		var counted int

		for _, path := range pkg.files {
			metric := r.files[path]
			if metric.Test {
				test.add(*metric)
				continue
			}
			code.add(*metric)
			counted++
		}

		all.add(code)
		tests.add(test)
		files += counted

		rows = append(rows, []string{
			pkg.importPath,
			strconv.Itoa(counted),
			strconv.Itoa(code.Types),
			strconv.Itoa(code.TypesSelf),
			strconv.Itoa(code.Funcs),
			strconv.Itoa(code.FuncsSelf),
			share(code),
			share(test),
		})
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })

	return model.NewStatistics(
		[]string{"Package", "Files", "Types", "Types(s)", "Funcs", "Funcs(s)", "Coupling", "Tests"},
		rows,
		model.HeaderText("What each file needs from the rest of its package. Types(s) and Funcs(s) are the ones reaching nothing outside the file they are in."),
		model.FooterText(fmt.Sprintf("%d packages of two or more files, %d files, %d symbols, %d reaching another file, %s.",
			len(r.packages), files, all.Symbols, all.Coupled, share(all))),
	)
}

// mostCoupled names the files reaching furthest outside themselves, which are
// the ones to read first. The count comes before the share, so a file of two
// symbols does not head the table for having both of them coupled.
func (r Results) mostCoupled() model.Statistics {
	paths := make([]string, len(r.order))
	copy(paths, r.order)

	sort.SliceStable(paths, func(i, j int) bool {
		left, right := r.files[paths[i]], r.files[paths[j]]
		if left.Coupled != right.Coupled {
			return left.Coupled > right.Coupled
		}
		return paths[i] < paths[j]
	})
	if len(paths) > coupledFiles {
		paths = paths[:coupledFiles]
	}

	rows := make([][]string, 0, len(paths))
	for _, path := range paths {
		metric := r.files[path]
		rows = append(rows, []string{
			path,
			strconv.Itoa(metric.Symbols),
			strconv.Itoa(metric.Coupled),
			share(*metric),
			mark(metric.Test),
		})
	}

	return model.NewStatistics(
		[]string{"File", "Symbols", "Coupled", "Coupling", "Test"},
		rows,
		model.HeaderText("The files reaching furthest into the rest of their package."),
	)
}

// share reads a coupling as a percentage, and reads as nothing for a file or a
// package that declares nothing to couple.
func share(metric Metric) string {
	if metric.Symbols == 0 {
		return "none"
	}
	return fmt.Sprintf("%.1f%%", metric.Coupling())
}

// mark renders a flag as a column reads it.
func mark(yes bool) string {
	if yes {
		return "yes"
	}
	return ""
}
