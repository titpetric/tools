package prose

import (
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/model"
)

func TestShapeMessage(t *testing.T) {
	for name, one := range map[string]struct {
		finding Finding
		want    []string
		absent  []string
	}{
		"a shape the baseline avoids": {
			Finding{Skeleton: "beside", Phrase: "beside", Count: 9, Absent: true},
			[]string{`"beside"`, "9 times", "never writes it"},
			[]string{"X stands for", "(\"beside\")"},
		},
		"a shape it writes less often": {
			Finding{Skeleton: "is what", Phrase: "is what", Count: 1, Ratio: 42},
			[]string{`"is what"`, "1 time,", "42x the baseline's rate"},
			[]string{"X stands for"},
		},
		"a shape holding subject matter": {
			Finding{Skeleton: "is what X", Phrase: "is what the", Count: 4, Ratio: 12},
			[]string{`"is what X"`, `("is what the")`, "X stands for a word of the subject matter"},
			nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := shapeMessage(one.finding)
			for _, want := range one.want {
				if !strings.Contains(got, want) {
					t.Errorf("the message is missing %q: %q", want, got)
				}
			}
			for _, unwanted := range one.absent {
				if strings.Contains(got, unwanted) {
					t.Errorf("the message holds %q, which says nothing here: %q", unwanted, got)
				}
			}
		})
	}
}

func TestResult_Issue(t *testing.T) {
	issue := Result{
		Rule:     RuleDensity,
		Message:  "a sentence runs long",
		Position: model.Position{File: "a.go", Line: 12},
	}.Issue()

	if issue.Linter != Name || issue.Rule != RuleDensity {
		t.Errorf("the issue reads %+v", issue)
	}
	if issue.Severity != model.SeverityWarn {
		t.Errorf("the severity is %v, want a warning", issue.Severity)
	}
	if issue.Position.File != "a.go" || issue.Position.Line != 12 {
		t.Errorf("the position reads %+v", issue.Position)
	}
	if issue.Message != "a sentence runs long" {
		t.Errorf("the message reads %q", issue.Message)
	}
}

func TestResultsOfNothing(t *testing.T) {
	results := &Results{}

	if results.Linter() != Name {
		t.Errorf("Linter() = %q, want %q", results.Linter(), Name)
	}
	if results.Len() != 0 {
		t.Errorf("Len() = %d, want 0", results.Len())
	}
	if stats := results.Statistics(); stats != nil {
		t.Errorf("Statistics() of nothing = %+v, want none", stats)
	}
	if metrics := results.Metrics(); len(metrics.Packages) != 0 {
		t.Errorf("Metrics() of nothing = %+v, want none", metrics)
	}
	for range results.All() {
		t.Error("All() walked a finding of an empty report")
	}
}

func TestResultsStatisticsAndMetrics(t *testing.T) {
	results := &Results{
		baseline: "a baseline",
		tree:     Density{Blocks: 2, Words: 40, BlockWords: 20, SentenceWords: 18, P90Words: 30, Subordinators: 20, Commas: 50, Stacked: 50},
		corpus:   Density{Blocks: 200, Words: 4000, BlockWords: 10, SentenceWords: 9, P90Words: 23, Subordinators: 10, Commas: 40, Stacked: 25},
	}
	results.add(Result{Rule: RuleShape, Message: "a shape", Position: model.Position{File: "a.go", Line: 3}})

	stats := results.Statistics()
	if len(stats) != 1 {
		t.Fatalf("Statistics() = %d tables, want 1", len(stats))
	}
	if !strings.Contains(stats[0].Header, "a baseline") {
		t.Errorf("the header names no baseline: %q", stats[0].Header)
	}
	if !strings.Contains(stats[0].Footer, "1 finding") {
		t.Errorf("the footer miscounts: %q", stats[0].Footer)
	}

	var rows string
	for _, row := range stats[0].Rows {
		rows += strings.Join(row, " ") + "\n"
	}
	for _, want := range []string{"2.0x", "Median words per block", "Blocks stacking subordinators"} {
		if !strings.Contains(rows, want) {
			t.Errorf("the table is missing %q:\n%s", want, rows)
		}
	}

	if metrics := results.Metrics(); len(metrics.Packages) == 0 {
		t.Error("Metrics() answered nothing for a measured tree")
	}

	var walked int
	for issue := range results.All() {
		walked++
		if issue.Linter != Name || issue.Severity != model.SeverityWarn {
			t.Errorf("the issue reads %+v", issue)
		}
	}
	if walked != 1 {
		t.Errorf("All() walked %d issues, want 1", walked)
	}
}

func TestOver(t *testing.T) {
	for name, one := range map[string]struct {
		part, whole float64
		want        string
	}{
		"twice":           {20, 10, "2.0x"},
		"no whole":        {20, 0, ""},
		"no part":         {0, 10, ""},
		"like its corpus": {10, 10, "1.0x"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := over(one.part, one.whole); got != one.want {
				t.Errorf("over(%v, %v) = %q, want %q", one.part, one.whole, got, one.want)
			}
		})
	}
}

func TestPlural(t *testing.T) {
	if got := plural(1, "finding", "findings"); got != "1 finding" {
		t.Errorf("plural(1) = %q", got)
	}
	if got := plural(2, "finding", "findings"); got != "2 findings" {
		t.Errorf("plural(2) = %q", got)
	}
}
