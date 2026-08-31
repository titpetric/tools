package filecheck

import (
	"fmt"
	"iter"
	"sort"
	"strconv"

	"github.com/titpetric/tools/splint/model"
)

// lengthBuckets are the upper bounds of the length histogram, doubling the way
// the gofsck size histogram does, so a file moving up a bucket is a file that
// grew by as much as it already was. The threshold sits on a boundary, which
// makes the reported files exactly the two rightmost buckets.
var lengthBuckets = []int{50, 100, 200, 400, 800}

// longestFiles is how many of the longest files the second table names. It is
// a list to act on rather than an inventory: past ten nobody reads further.
const longestFiles = 10

// Result is one long file, in the terms this linter thinks in.
type Result struct {
	// Rule is which of the linter's rules the finding is under.
	Rule string

	// Symbol is the filename, because a file is what the finding is about,
	// and Position is the file within its package directory.
	Symbol   string
	Position model.Position

	// Message says how long the file ran to.
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

// Results is what the linter found and what it measured.
type Results struct {
	findings []Result
	files    map[string]*Metric
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

// Metrics is what the linter measured, per file.
func (r Results) Metrics() model.LintMetrics {
	metrics := model.LintMetrics{}
	for path, metric := range r.files {
		metrics.AddFile(path, *metric)
	}
	return metrics
}

// Statistics is how the lengths read: how the tree is spread across the
// buckets, and then the files a reader would split first.
func (r Results) Statistics() []model.Statistics {
	if len(r.order) == 0 {
		return nil
	}
	return []model.Statistics{r.histogram(), r.longest()}
}

// histogram is the spread of file lengths, one row per bucket. It is the table
// that says what kind of tree this is: weight on the left is a tree of small
// files, weight on the right is a tree nobody reads in one sitting.
func (r Results) histogram() model.Statistics {
	counts := make([]int, len(lengthBuckets)+1)
	var lines, longest int

	for _, path := range r.order {
		metric := r.files[path]
		counts[bucket(metric.Lines)]++
		lines += metric.Lines
		if metric.Lines > longest {
			longest = metric.Lines
		}
	}

	rows := make([][]string, 0, len(counts))
	for index, count := range counts {
		rows = append(rows, []string{
			bucketLabel(index),
			strconv.Itoa(count),
			percent(count, len(r.order)),
		})
	}

	return model.NewStatistics(
		[]string{"Lines", "Files", "Share"},
		rows,
		model.HeaderText("How the files of the tree are spread by length."),
		model.FooterText(fmt.Sprintf("%d files, %d lines of code, %d on average, longest %d, %d over %d.",
			len(r.order), lines, lines/len(r.order), longest, len(r.findings), maxLines)),
	)
}

// longest names the files at the top of the distribution, whether or not they
// were reported: a tree under the threshold still has a longest file, and that
// is the one that grows into a finding next.
func (r Results) longest() model.Statistics {
	paths := make([]string, len(r.order))
	copy(paths, r.order)
	sort.SliceStable(paths, func(i, j int) bool {
		left, right := r.files[paths[i]], r.files[paths[j]]
		if left.Lines != right.Lines {
			return left.Lines > right.Lines
		}
		return paths[i] < paths[j]
	})
	if len(paths) > longestFiles {
		paths = paths[:longestFiles]
	}

	rows := make([][]string, 0, len(paths))
	for _, path := range paths {
		metric := r.files[path]
		rows = append(rows, []string{
			path,
			strconv.Itoa(metric.Lines),
			strconv.Itoa(metric.Size),
			mark(metric.Test),
			mark(metric.Long),
		})
	}

	return model.NewStatistics(
		[]string{"File", "Lines", "Bytes", "Test", "Reported"},
		rows,
		model.HeaderText("The longest files, which are the ones to split first."),
	)
}

// count records one file, and returns what a finding on it is recorded
// against. It returns nil for a file already measured: a directory holding an
// external test package is two definitions of the same path, and a file
// counted twice would be two rows of one histogram bucket.
func (r *Results) count(path string, file model.File) *Metric {
	if r.files == nil {
		r.files = map[string]*Metric{}
	}
	if _, known := r.files[path]; known {
		return nil
	}

	metric := &Metric{Lines: file.Lines, Size: file.Size, Test: file.Test}
	r.files[path] = metric
	r.order = append(r.order, path)
	return metric
}

// add records one finding against the file it was found in.
func (r *Results) add(metric *Metric, result Result) {
	r.findings = append(r.findings, result)
	metric.Long = true
}

// bucket is the histogram bucket a length falls in. Anything past the last
// bound lands in the open bucket above it.
func bucket(lines int) int {
	for index, bound := range lengthBuckets {
		if lines <= bound {
			return index
		}
	}
	return len(lengthBuckets)
}

// bucketLabel is the range a bucket covers, read as a reader would say it.
func bucketLabel(index int) string {
	if index == len(lengthBuckets) {
		return strconv.Itoa(lengthBuckets[index-1]+1) + "+"
	}
	low := 1
	if index > 0 {
		low = lengthBuckets[index-1] + 1
	}
	return strconv.Itoa(low) + "-" + strconv.Itoa(lengthBuckets[index])
}

// mark renders a flag as a column, and reads as nothing when it is not set.
func mark(set bool) string {
	if set {
		return "yes"
	}
	return "-"
}

// percent renders a share, and reads as nothing when there was nothing to take
// a share of.
func percent(part, whole int) string {
	if whole == 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", float64(part)/float64(whole)*100)
}
