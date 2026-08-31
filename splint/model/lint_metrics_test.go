package model

import (
	"encoding/json"
	"strings"
	"testing"
)

// metric stands in for whatever a linter measures, which the framework carries
// and does not read.
type metric struct {
	Files  int `json:"Files"`
	Passed int `json:"Passed"`
}

func TestLintMetrics(t *testing.T) {
	var metrics LintMetrics

	if !metrics.Empty() {
		t.Error("Empty() said the zero value holds something")
	}

	// The zero value takes an entry without being made first, since a linter
	// that measures one thing should not have to say so up front.
	metrics.AddPackage("example.com/x", metric{Files: 3, Passed: 2})
	metrics.AddFile("x/y.go", metric{Files: 1})

	if metrics.Empty() {
		t.Error("Empty() said filled metrics hold nothing")
	}
	if got := metrics.PackageKeys(); len(got) != 1 || got[0] != "example.com/x" {
		t.Errorf("PackageKeys() = %v", got)
	}
	if got := metrics.FileKeys(); len(got) != 1 || got[0] != "x/y.go" {
		t.Errorf("FileKeys() = %v", got)
	}
}

func TestLintMetricsKeysAreOrdered(t *testing.T) {
	metrics := NewLintMetrics()
	for _, name := range []string{"c", "a", "b"} {
		metrics.AddPackage(name, metric{})
		metrics.AddFile(name, metric{})
	}

	// The keys come back in order, so a rendering of the metrics reads the
	// same every time.
	if got := strings.Join(metrics.PackageKeys(), ","); got != "a,b,c" {
		t.Errorf("PackageKeys() = %q", got)
	}
	if got := strings.Join(metrics.FileKeys(), ","); got != "a,b,c" {
		t.Errorf("FileKeys() = %q", got)
	}
}

// TestLintMetricsEncode covers what the metrics are for: a linter's own type
// carried out to a document without the framework knowing what it is.
func TestLintMetricsEncode(t *testing.T) {
	metrics := LintMetrics{}
	metrics.AddPackage("example.com/x", metric{Files: 3, Passed: 2})

	data, err := json.Marshal(metrics)
	if err != nil {
		t.Fatal(err)
	}

	got := string(data)
	if !strings.Contains(got, `"Files":3`) || !strings.Contains(got, `"Passed":2`) {
		t.Errorf("Marshal() = %s", got)
	}
	// A half nobody filled is left out rather than written as null.
	if strings.Contains(got, "Files\":{}") || strings.Contains(got, `"Files":null`) {
		t.Errorf("Marshal() wrote an empty half: %s", got)
	}
}
