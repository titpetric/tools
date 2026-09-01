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

// Breakdown is what one linter found: how many at each level, and how many
// under each of its rules.
type Breakdown struct {
	// Linter is the linter the findings came from, and Total is how many of
	// them there are.
	Linter string `json:"Linter" yaml:"Linter"`
	Total  int    `json:"Total" yaml:"Total"`

	// Errors, Warnings and Notices are the findings by level, which is what a
	// reader scans before reading any of them.
	Errors   int `json:"Errors" yaml:"Errors"`
	Warnings int `json:"Warnings" yaml:"Warnings"`
	Notices  int `json:"Notices" yaml:"Notices"`

	// Rules is how many findings each rule of the linter reported, most first.
	Rules []RuleCount `json:"Rules,omitempty" yaml:"Rules,omitempty"`
}

// RuleCount is one rule and how much it had to say.
type RuleCount struct {
	Rule  string `json:"Rule" yaml:"Rule"`
	Count int    `json:"Count" yaml:"Count"`
}

// Breakdowns is what each linter that found something reported, in the order
// the linters ran.
//
// A linter that found nothing is left out: the run says which linters ran, and
// a row of zeroes for each of them is a table of what did not happen.
func (r *Report) Breakdowns() []Breakdown {
	at := map[string]*Breakdown{}
	rules := map[string]map[string]int{}

	for _, issue := range r.Issues {
		one, known := at[issue.Linter]
		if !known {
			one = &Breakdown{Linter: issue.Linter}
			at[issue.Linter] = one
			rules[issue.Linter] = map[string]int{}
		}

		one.Total++
		switch {
		case issue.Severity >= model.SeverityError:
			one.Errors++
		case issue.Severity >= model.SeverityWarn:
			one.Warnings++
		default:
			one.Notices++
		}

		rule := issue.Rule
		if rule == "" {
			rule = issue.Linter
		}
		rules[issue.Linter][rule]++
	}

	var out []Breakdown
	for _, linter := range r.Linters {
		one, found := at[linter]
		if !found {
			continue
		}
		one.Rules = counted(rules[linter])
		out = append(out, *one)
	}

	return out
}

// Quiet names the linters that ran and found nothing, in the order they ran.
func (r *Report) Quiet() []string {
	found := map[string]bool{}
	for _, issue := range r.Issues {
		found[issue.Linter] = true
	}

	var out []string
	for _, linter := range r.Linters {
		if !found[linter] {
			out = append(out, linter)
		}
	}
	return out
}

// counted orders the rules of one linter, the loudest first and by name where
// two said as much as each other.
func counted(rules map[string]int) []RuleCount {
	out := make([]RuleCount, 0, len(rules))
	for rule, count := range rules {
		out = append(out, RuleCount{Rule: rule, Count: count})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Rule < out[j].Rule
	})

	return out
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
