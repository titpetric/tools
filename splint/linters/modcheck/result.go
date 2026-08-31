package modcheck

import (
	"context"
	"fmt"
	"iter"
	"sort"
	"strconv"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/modproxy"
)

// Result is one finding about the dependencies of a module.
type Result struct {
	// Rule is which of the module rules the finding is under, and Severity is
	// how much it means: a replace fails a run, a thin dependency is a
	// suggestion.
	Rule     string
	Severity model.Severity

	// Symbol is the module the finding is about, and Message says what is
	// wrong with it.
	Symbol  string
	Message string
}

// Issue renders the finding as the framework reads it.
//
// A module is not in a file, so the position names the go.mod it came from
// rather than a line of source: that is where a reader goes to act on any of
// this.
func (r Result) Issue() model.Issue {
	return model.Issue{
		Linter:   Name,
		Rule:     r.Rule,
		Severity: r.Severity,
		Position: model.Position{File: "go.mod"},
		Symbol:   r.Symbol,
		Message:  r.Message,
	}
}

// Results is what the linter found and what it measured.
type Results struct {
	findings []Result

	// deps is every dependency, keyed by module path, and modules is how many
	// go.mod files the document was parsed from.
	deps    map[string]*Dependency
	modules int
}

// add records one finding.
func (r *Results) add(result Result) {
	r.findings = append(r.findings, result)
}

// ask fills in what only the proxy knows, and leaves it blank when the proxy
// could not be reached. A report is worth having either way.
func (r *Results) ask(ctx context.Context, proxy *modproxy.Client, deps map[string]*Dependency) {
	if proxy == nil || len(deps) == 0 {
		return
	}

	versions := make(map[string]string, len(deps))
	for path, dep := range deps {
		if dep.Version != "" {
			versions[path] = dep.Version
		}
	}

	for path, info := range proxy.LookupAll(ctx, versions) {
		dep, known := deps[path]
		if !known {
			continue
		}
		dep.Size = info.Size
		if info.Behind() {
			dep.Latest = info.Latest
		}
	}
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

// Metrics is what the linter measured, one entry per dependency.
//
// The key is the module path, which is what a dependency is called everywhere
// else. They are packages rather than files because a module is neither: of
// the two halves the model offers, a dependency is much closer to a package
// than to a file on disk.
func (r Results) Metrics() model.LintMetrics {
	metrics := model.LintMetrics{}
	for path, dep := range r.deps {
		metrics.AddPackage(path, *dep)
	}
	return metrics
}

// Statistics is the dependency table, the ones a consumer links first.
func (r Results) Statistics() []model.Statistics {
	deps := sorted(r.deps)
	if len(deps) == 0 {
		return nil
	}

	rows := make([][]string, 0, len(deps))
	var size int64
	var shipped, unused int

	for _, dep := range deps {
		size += dep.Size
		switch {
		case dep.Shipped():
			shipped++
		case dep.Files == 0 && !dep.Indirect:
			unused++
		}

		rows = append(rows, []string{
			dep.Path,
			dep.Version,
			bytes(dep.Size),
			strconv.Itoa(dep.Files),
			strconv.Itoa(dep.Packages),
			strconv.Itoa(dep.Symbols),
			dep.Kind(),
			dep.Latest,
		})
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })

	return []model.Statistics{model.NewStatistics(
		[]string{"Import", "Version", "Size", "Files", "Pkgs", "Symbols", "Kind", "Behind"},
		rows,
		model.HeaderText(header(r.modules)),
		model.FooterText(footer(len(deps), shipped, unused, size)),
	)}
}

// footer is the one line a reader takes away. The size is left out when the
// proxy answered for nothing, since a total of nothing is not a total.
func footer(deps, shipped, unused int, size int64) string {
	out := fmt.Sprintf("%d dependencies, %d linked by a consumer", deps, shipped)
	if size > 0 {
		out += ", " + bytes(size)
	}
	return fmt.Sprintf("%s, %d required and unimported.", out, unused)
}

// header says what the table is about, counting the modules it read.
func header(modules int) string {
	if modules == 1 {
		return "What the module depends on, and how far each dependency reaches in."
	}
	return fmt.Sprintf("What %d modules depend on, and how far each dependency reaches in.", modules)
}

// bytes renders a size the way a reader reads one, and reads as nothing when
// the proxy could not say.
func bytes(size int64) string {
	switch {
	case size <= 0:
		return "-"
	case size >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(size)/(1<<20))
	case size >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(size)/(1<<10))
	}
	return fmt.Sprintf("%d B", size)
}
