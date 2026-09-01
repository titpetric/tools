package modcheck

import (
	"context"
	"fmt"
	"iter"
	"sort"
	"strconv"
	"strings"

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

	// Position is the file the finding is in, and is empty for a finding about
	// the module rather than about a line of source.
	Position model.Position
}

// Issue renders the finding as the framework reads it.
//
// A module is not in a file, so a finding about one names the go.mod it came
// from: that is where a reader goes to act on it. A finding about an import is
// in the file that writes it, and says so.
func (r Result) Issue() model.Issue {
	position := r.Position
	if position.File == "" {
		position.File = "go.mod"
	}

	return model.Issue{
		Linter:   Name,
		Rule:     r.Rule,
		Severity: r.Severity,
		Position: position,
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

	// sums is every version go.sum records, which is what the versions of one
	// module are counted from, and sizes is what the proxy answered about
	// each, keyed "path@version".
	sums  []model.Sum
	sizes map[string]int64
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

	r.sizes = map[string]int64{}
	for path, info := range proxy.LookupAll(ctx, versions) {
		r.sizes[path+"@"+info.Version] = info.Size

		dep, known := deps[path]
		if !known {
			continue
		}
		dep.Size = info.Size
		dep.requires = info.Requires
		if info.Behind() {
			dep.Latest = info.Latest
		}
	}

	// The versions go.sum records are asked one question and not four: what a
	// copy of the source weighs. When it was published and what it requires
	// say nothing about that.
	unasked := r.unasked()
	for path, size := range proxy.SizeAll(ctx, unasked) {
		r.sizes[path+"@"+unasked[path]] = size
	}

	reach(deps)
}

// unasked is every version go.sum records the source of that the requirements
// did not already cover, keyed by path.
//
// A module linked at two majors is two downloads under two paths, and neither
// need be a requirement of the module being read: the second major can arrive
// through a dependency. Asking about them is what puts a size on the copies.
func (r *Results) unasked() map[string]string {
	out := map[string]string{}

	for _, sum := range r.sums {
		if !sum.Zip {
			continue
		}
		if _, known := r.sizes[sum.Path+"@"+sum.Version]; known {
			continue
		}
		if _, asked := out[sum.Path]; asked {
			continue
		}
		out[sum.Path] = sum.Version
	}

	return out
}

// reach walks the module graph and records what each dependency drags in
// behind it.
//
// The walk stops at the modules this build carries. A dependency requires
// things this build resolved away, and counting those would report a cost
// nobody pays; what is worth knowing is how much of the build list is here
// because of one requirement.
func reach(deps map[string]*Dependency) {
	// A run that asked nobody knows nothing about the graph, and a reach of
	// nothing is a measurement rather than an absence. It is left blank.
	answered := false
	for _, dep := range deps {
		if len(dep.requires) > 0 {
			answered = true
			break
		}
	}
	if !answered {
		return
	}

	for path, dep := range deps {
		seen := map[string]bool{path: true}
		queue := append([]string(nil), dep.requires...)

		dep.Weight = dep.Size
		for len(queue) > 0 {
			next := queue[0]
			queue = queue[1:]
			if seen[next] {
				continue
			}
			seen[next] = true

			other, carried := deps[next]
			if !carried {
				continue
			}
			dep.Reach++
			dep.Weight += other.Size
			queue = append(queue, other.requires...)
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

// Statistics is the dependency table and, where go.sum records more than one
// version of a module, the table of those.
func (r Results) Statistics() []model.Statistics {
	deps := sorted(r.deps)
	if len(deps) == 0 {
		return nil
	}

	out := []model.Statistics{r.dependencies(deps)}
	if table, ok := r.repeated(); ok {
		out = append(out, table)
	}
	return out
}

// dependencies is the table of what the module depends on.
func (r Results) dependencies(deps []*Dependency) model.Statistics {
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
			reached(dep.Reach, dep.Weight),
			bytes(dep.Weight),
			strconv.Itoa(dep.Files),
			strconv.Itoa(dep.Packages),
			strconv.Itoa(dep.Symbols),
			dep.Kind(),
			dep.Latest,
		})
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })

	return model.NewStatistics(
		[]string{"Import", "Version", "Size", "Deps", "Total", "Files", "Pkgs", "Symbols", "Kind", "Behind"},
		rows,
		model.HeaderText(header(r.modules)),
		model.FooterText(footer(len(deps), shipped, unused, size)),
	)
}

// repeated is the table of modules go.sum records at more than one version,
// and reports whether there are any.
func (r Results) repeated() (model.Statistics, bool) {
	found := duplicates(r.sums, r.sizes)
	if len(found) == 0 {
		return model.Statistics{}, false
	}

	rows := make([][]string, 0, len(found))
	var overhead int64
	var linked int

	for _, entry := range found {
		overhead += entry.Overhead
		if len(entry.Linked) > 1 {
			linked++
		}

		rows = append(rows, []string{
			entry.Base,
			strconv.Itoa(entry.Versions),
			strings.Join(entry.Linked, ", "),
			bytes(entry.Size),
			bytes(entry.Overhead),
		})
	}

	return model.NewStatistics(
		[]string{"Module", "Versions", "Linked", "Size", "Overhead"},
		rows,
		model.HeaderText("Modules go.sum records at more than one version. A version that is not linked was read for its requirements and not downloaded."),
		model.FooterText(repeatedFooter(len(found), linked, overhead)),
	), true
}

// repeatedFooter counts the modules recorded more than once, and the bytes of
// the copies past the first where there are any.
func repeatedFooter(modules, linked int, overhead int64) string {
	out := fmt.Sprintf("%d modules recorded at more than one version, %d linked more than once", modules, linked)
	if overhead > 0 {
		out += ", " + bytes(overhead) + " of it linked twice or more"
	}
	return out + "."
}

// reached renders how many modules a dependency brings with it, and reads as
// nothing where the graph was not read: a run that asked nobody reports no
// reach rather than a reach of nothing.
func reached(count int, weight int64) string {
	if weight == 0 {
		return "-"
	}
	return strconv.Itoa(count)
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
