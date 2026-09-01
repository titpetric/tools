// Package report renders what the linters found.
//
// One issue always reads the same way, "path/file.go:12: linter: message",
// which is the shape a compiler writes and the shape GitHub Actions resolves
// against a checkout. What changes is the frame around it: a terminal gets
// colour, and anything else gets a markdown table.
package report

import (
	"sort"

	"github.com/titpetric/tools/splint/model"
)

// Report is every issue of one run, gathered from the linters that ran.
//
// The fields carry a json and a yaml tag naming the same key, so a run asked
// for its findings as data answers the same thing in either encoding. A
// severity writes itself as the word it is called in both, because a level is
// a text marshaller and both encoders read one.
type Report struct {
	// Issues are the findings, sorted by where they are.
	Issues []model.Issue `json:"Issues" yaml:"Issues"`

	// Linters names the linters that ran, in the order they ran.
	Linters []string `json:"Linters" yaml:"Linters"`
}

// New gathers the reports of a run into one.
func New(reports ...model.LintReport) *Report {
	report := &Report{}
	for _, one := range reports {
		if one == nil {
			continue
		}
		report.Linters = append(report.Linters, one.Linter())
		for issue := range one.All() {
			report.Issues = append(report.Issues, issue)
		}
	}
	report.sort()
	return report
}

// Len is how many issues the report holds.
func (r *Report) Len() int {
	return len(r.Issues)
}

// Worst is the highest severity in the report, and is the debug level for a
// report holding nothing.
func (r *Report) Worst() model.Severity {
	worst := model.SeverityDebug
	for _, issue := range r.Issues {
		if issue.Severity > worst {
			worst = issue.Severity
		}
	}
	return worst
}

// Counts is how many issues each linter reported, which is what a summary
// line reads.
func (r *Report) Counts() map[string]int {
	counts := make(map[string]int, len(r.Linters))
	for _, issue := range r.Issues {
		counts[issue.Linter]++
	}
	return counts
}

// sort orders the issues by file, then line, then linter, so a report reads
// down a tree rather than jumping between linters.
func (r *Report) sort() {
	sort.SliceStable(r.Issues, func(i, j int) bool {
		a, b := r.Issues[i], r.Issues[j]
		if a.Position.File != b.Position.File {
			return a.Position.File < b.Position.File
		}
		if a.Position.Line != b.Position.Line {
			return a.Position.Line < b.Position.Line
		}
		if a.Linter != b.Linter {
			return a.Linter < b.Linter
		}
		return a.Message < b.Message
	})
}

// Line renders one issue as a path a tool resolves, which is what GitHub
// Actions reads out of a log and turns into an annotation.
func Line(issue model.Issue) string {
	return issue.Position.Ref() + ": " + issue.RuleName() + ": " + issue.Message
}
