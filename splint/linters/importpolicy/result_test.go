package importpolicy

import (
	"testing"

	"github.com/titpetric/tools/splint/model"
)

// TestResult_Issue covers the finding as the framework reads it: the linter
// and the rule it came from, a warning, and the replacement as an attribute a
// reader asks for by name.
func TestResult_Issue(t *testing.T) {
	result := Result{
		Rule:        RuleDenied,
		Symbol:      "gopkg.in/yaml.v3",
		Position:    model.Position{Package: "config", File: "config/config.go"},
		Replacement: "github.com/goccy/go-yaml",
		Message:     "a message",
	}

	issue := result.Issue()
	if issue.Linter != Name || issue.Rule != RuleDenied {
		t.Errorf("issue = %s/%s, want %s/%s", issue.Linter, issue.Rule, Name, RuleDenied)
	}
	if issue.Severity != model.SeverityWarn {
		t.Errorf("severity = %v, want a warning", issue.Severity)
	}
	if issue.Symbol != result.Symbol || issue.Message != result.Message {
		t.Errorf("issue = %+v, want the result's symbol and message", issue)
	}
	if got, ok := issue.Attr("replacement"); !ok || got != result.Replacement {
		t.Errorf("replacement = %q, want %q", got, result.Replacement)
	}

	result.Replacement = ""
	if got, ok := result.Issue().Attr("replacement"); ok {
		t.Errorf("replacement = %q, want none when the tree names none", got)
	}
}

// TestResults covers the report: the linter it names, the count, the issues it
// walks, and the two measurements it answers with nothing.
func TestResults(t *testing.T) {
	var results Results
	if results.Linter() != Name {
		t.Errorf("Linter() = %q, want %q", results.Linter(), Name)
	}
	if results.Len() != 0 {
		t.Errorf("Len() = %d on an empty report, want 0", results.Len())
	}

	results.add(Result{Rule: RuleDenied, Symbol: "gopkg.in/yaml.v2"})
	results.add(Result{Rule: RuleDenied, Symbol: "gopkg.in/check.v1"})

	if results.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", results.Len())
	}
	issues := model.Issues(results)
	if len(issues) != 2 || issues[0].Symbol != "gopkg.in/yaml.v2" || issues[1].Symbol != "gopkg.in/check.v1" {
		t.Errorf("All() walked %+v, want both findings in the order they were added", issues)
	}

	// The walk stops where the reader stops, which is what a range over the
	// sequence does on break.
	count := 0
	for range results.All() {
		count++
		break
	}
	if count != 1 {
		t.Errorf("the walk yielded %d after a break, want 1", count)
	}

	if metrics := results.Metrics(); len(metrics.Files) != 0 || len(metrics.Packages) != 0 {
		t.Errorf("Metrics() = %+v, want nothing", metrics)
	}
	if results.Statistics() != nil {
		t.Errorf("Statistics() = %v, want nothing", results.Statistics())
	}
}
