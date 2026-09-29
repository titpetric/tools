package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRenderVerdictStats(t *testing.T) {
	first := sampleVerdict()
	second := sampleVerdict()
	second.Version, second.Since = "v1.0.0", "v0.9.0"
	second.API = apiDiff{}
	second.Commits = nil

	var out bytes.Buffer
	renderVerdictStats(&out, []verdict{first, second}, false)

	got := out.String()
	for _, want := range []string{
		// The module is named once, without a version: the table holds them.
		"# example.com/x\n",
		"| Version | Since | Commits | Symbols + | Symbols ~ | Symbols - | Fields + | Fields ~ | Fields - |",
		"| v1.1.0 | v1.0.0 | 2 | 1 | 1 | 1 | 1 | 1 | 1 |",
		"| v1.0.0 | v0.9.0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVerdictStats() output missing %q:\n%s", want, got)
		}
	}

	// The analysis is what --stats collapses, so none of it is written.
	for _, unwanted := range []string{"## Commits", "## API", "## Data model", "Minor release"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("renderVerdictStats() wrote %q, which it collapses:\n%s", unwanted, got)
		}
	}
}

func TestRenderVerdictStatsWithNothingToReport(t *testing.T) {
	var out bytes.Buffer
	renderVerdictStats(&out, nil, false)
	if got := out.String(); got != "" {
		t.Errorf("renderVerdictStats() of no releases = %q, want nothing", got)
	}
}

func TestParseOptionsVerdictStats(t *testing.T) {
	tests := []struct {
		args  []string
		stats bool
		chain bool
	}{
		{args: []string{"worktree", "verdict", "--stats"}, stats: true},
		{args: []string{"worktree", "verdict", "--stats", "--all"}, stats: true, chain: true},
		{args: []string{"worktree", "verdict"}},
	}

	for _, test := range tests {
		func() {
			args, commandLine := os.Args, flag.CommandLine
			t.Cleanup(func() { os.Args, flag.CommandLine = args, commandLine })

			os.Args = test.args
			flag.CommandLine = flag.NewFlagSet("worktree", flag.ContinueOnError)
			flag.CommandLine.SetOutput(io.Discard)

			opts := ParseOptions()
			if opts.Stats != test.stats || opts.Chain != test.chain {
				t.Errorf("%v: {Stats: %v, Chain: %v}, want {%v, %v}", test.args, opts.Stats, opts.Chain, test.stats, test.chain)
			}
		}()
	}
}
