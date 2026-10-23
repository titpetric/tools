package integration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/titpetric/tools/splint/linters/integration"
	"github.com/titpetric/tools/splint/model"
)

func TestResult_Issue(t *testing.T) {
	t.Parallel()

	result := integration.Result{
		Rule:     integration.RuleBoundary,
		Severity: model.SeverityWarn,
		Symbol:   "TestOpen",
		Reason:   "opens sqlite \":memory:\"",
		Message:  "TestOpen opens sqlite",
	}
	issue := result.Issue()
	assert.Equal(t, integration.Name, issue.Linter)
	assert.Equal(t, model.SeverityWarn, issue.Severity)
	reason, ok := issue.Attr("reason")
	assert.True(t, ok)
	assert.Equal(t, result.Reason, reason)

	_, ok = integration.Result{Rule: integration.RuleTag}.Issue().Attr("reason")
	assert.False(t, ok, "a finding with no reason carries no attribute")
}

func TestResults(t *testing.T) {
	t.Parallel()

	var results integration.Results
	assert.Equal(t, integration.Name, results.Linter())
	assert.Equal(t, 0, results.Len())
	for range results.All() {
		t.Error("an empty report yields an issue")
	}
	assert.Empty(t, results.Metrics().Packages)
	assert.Empty(t, results.Statistics(), "an empty report renders no table")
}

func TestLinter_Name(t *testing.T) {
	t.Parallel()

	assert.Equal(t, integration.Name, integration.New().Name())
}
