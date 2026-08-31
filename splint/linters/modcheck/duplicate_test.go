package modcheck_test

import (
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/model"
)

// TestLinter_StatisticsRepeated covers the second table: what go.sum records
// more than one version of, and which of those versions the build downloads.
func TestLinter_StatisticsRepeated(t *testing.T) {
	root := document()
	root.Modules[0].Sums = []model.Sum{
		// Two majors of one module, both downloaded.
		{Path: "example.com/deep", Version: "v1.0.0", Zip: true},
		{Path: "example.com/deep/v2", Version: "v2.1.0", Zip: true},
		// Three versions offered of one, and the selected one downloaded.
		{Path: "example.com/thin", Version: "v0.1.0"},
		{Path: "example.com/thin", Version: "v0.2.0", Zip: true},
		{Path: "example.com/thin", Version: "v0.3.0"},
		// One version, which is not a repeat and is not reported.
		{Path: "example.com/unused", Version: "v0.1.0", Zip: true},
	}

	stats := lint(t, root).Statistics()
	if len(stats) != 2 {
		t.Fatalf("Statistics() = %d tables, want the dependencies and the repeats", len(stats))
	}

	table := stats[1]
	if len(table.Rows) != 2 {
		t.Fatalf("Rows = %#v, want deep and thin", table.Rows)
	}

	// Module, Versions, Linked, Size, Overhead.
	if got := table.Rows[0]; got[0] != "example.com/deep" || got[1] != "2" || got[2] != "v1.0.0, v2 v2.1.0" {
		t.Errorf("deep reads %#v", got)
	}
	if got := table.Rows[1]; got[0] != "example.com/thin" || got[1] != "3" || got[2] != "v0.2.0" {
		t.Errorf("thin reads %#v", got)
	}
	if !strings.Contains(table.Footer, "2 modules recorded at more than one version, 1 linked more than once") {
		t.Errorf("footer = %q", table.Footer)
	}
}
