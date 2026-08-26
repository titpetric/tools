package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chainString renders a chain as "from..to" per section, so a table test reads
// as the report it describes. The start of history and the working tree have no
// version to name, and are left empty on their side of the range.
func chainString(ranges []versionRange) []string {
	out := make([]string, 0, len(ranges))
	for _, r := range ranges {
		out = append(out, r.From+".."+r.To)
	}
	return out
}

func TestSeriesOpeners(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want []string
	}{{
		name: "one opener per series",
		tags: []string{"v0.0.1", "v0.0.2", "v0.1.0", "v0.1.1", "v0.2.0"},
		want: []string{"v0.0.1", "v0.1.0", "v0.2.0"},
	}, {
		name: "tags out of order",
		tags: []string{"v0.2.0", "v0.1.1", "v0.0.2", "v0.1.0", "v0.0.1"},
		want: []string{"v0.0.1", "v0.1.0", "v0.2.0"},
	}, {
		name: "a series that never tagged its zero",
		tags: []string{"v0.1.1", "v0.1.2", "v0.2.3"},
		want: []string{"v0.1.1", "v0.2.3"},
	}, {
		name: "a major bump opens a series of its own",
		tags: []string{"v0.9.0", "v0.9.1", "v1.0.0", "v1.0.1"},
		want: []string{"v0.9.0", "v1.0.0"},
	}, {
		name: "prereleases and other tags are not releases",
		tags: []string{"v0.1.0-rc.1", "v0.1.0", "nightly", "v0.2.0"},
		want: []string{"v0.1.0", "v0.2.0"},
	}, {
		name: "no tags at all",
		tags: nil,
		want: nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			for _, opener := range seriesOpeners(test.tags) {
				got = append(got, opener.String())
			}
			if strings.Join(got, " ") != strings.Join(test.want, " ") {
				t.Errorf("seriesOpeners(%v) = %v, want %v", test.tags, got, test.want)
			}
		})
	}
}

func TestReleaseChain(t *testing.T) {
	full := []string{"v0.0.1", "v0.0.2", "v0.1.0", "v0.1.1", "v0.2.0", "v0.2.1", "v0.2.2"}

	tests := []struct {
		name    string
		tags    []string
		ahead   bool
		verbose bool
		from    string
		upTo    string
		want    []string
	}{{
		name:  "series bumps, the latest release, and the working tree",
		tags:  full,
		ahead: true,
		want: []string{
			"v0.2.2..",
			"v0.2.0..v0.2.2",
			"v0.1.0..v0.2.0",
			"v0.0.1..v0.1.0",
			"..v0.0.1",
		},
	}, {
		name: "level with the latest tag, so there is nothing pending",
		tags: full,
		want: []string{
			"v0.2.0..v0.2.2",
			"v0.1.0..v0.2.0",
			"v0.0.1..v0.1.0",
			"..v0.0.1",
		},
	}, {
		name: "the latest release opens its own series",
		tags: []string{"v0.1.0", "v0.1.1", "v0.2.0"},
		want: []string{
			"v0.1.0..v0.2.0",
			"..v0.1.0",
		},
	}, {
		name:    "verbose reports every release against the one before it",
		tags:    []string{"v0.0.1", "v0.0.2", "v0.1.0"},
		verbose: true,
		want: []string{
			"v0.0.2..v0.1.0",
			"v0.0.1..v0.0.2",
			"..v0.0.1",
		},
	}, {
		name: "a series whose zero was never tagged",
		tags: []string{"v0.1.1", "v0.1.2", "v0.2.3"},
		want: []string{
			"v0.1.1..v0.2.3",
			"..v0.1.1",
		},
	}, {
		name: "a major bump is a series of its own",
		tags: []string{"v0.9.0", "v1.0.0", "v1.0.1"},
		want: []string{
			"v1.0.0..v1.0.1",
			"v0.9.0..v1.0.0",
			"..v0.9.0",
		},
	}, {
		name: "prereleases are not releases",
		tags: []string{"v0.1.0-rc.1", "v0.1.0", "v0.2.0-rc.1", "v0.2.0"},
		want: []string{
			"v0.1.0..v0.2.0",
			"..v0.1.0",
		},
	}, {
		name: "one release",
		tags: []string{"v0.1.0"},
		want: []string{"..v0.1.0"},
	}, {
		name:  "one release with work on top of it",
		tags:  []string{"v0.1.0"},
		ahead: true,
		want: []string{
			"v0.1.0..",
			"..v0.1.0",
		},
	}, {
		name:  "no releases at all is one range over the whole history",
		tags:  nil,
		ahead: true,
		want:  []string{".."},
	}, {
		name:  "bounded by a release, which is where the chain stops",
		tags:  full,
		ahead: true,
		upTo:  "v0.1.1",
		want: []string{
			"v0.1.0..v0.1.1",
			"v0.0.1..v0.1.0",
			"..v0.0.1",
		},
	}, {
		name:  "bounded by the release opening a series",
		tags:  full,
		ahead: true,
		upTo:  "v0.1.0",
		want: []string{
			"v0.0.1..v0.1.0",
			"..v0.0.1",
		},
	}, {
		name: "bounded above every release",
		tags: full,
		upTo: "v0.0.0",
		want: []string{"..v0.0.0"},
	}, {
		name:  "bounded below by a release, which is where the chain starts",
		tags:  full,
		ahead: true,
		from:  "v0.1.0",
		want: []string{
			"v0.2.2..",
			"v0.2.0..v0.2.2",
			"v0.1.0..v0.2.0",
		},
	}, {
		name:  "bounded below inside the latest series",
		tags:  full,
		ahead: true,
		from:  "v0.2.0",
		want: []string{
			"v0.2.2..",
			"v0.2.0..v0.2.2",
		},
	}, {
		name: "bounded at both ends",
		tags: full,
		from: "v0.0.1",
		upTo: "v0.2.0",
		want: []string{
			"v0.1.0..v0.2.0",
			"v0.0.1..v0.1.0",
		},
	}, {
		name: "bounded above every release there is",
		tags: full,
		from: "v9.0.0",
		want: nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := chainString(releaseChain(test.tags, test.ahead, test.verbose, test.from, test.upTo))
			if strings.Join(got, " | ") != strings.Join(test.want, " | ") {
				t.Errorf("releaseChain() =\n\t%v\nwant\n\t%v", got, test.want)
			}
		})
	}
}

// chainRepo builds a nested module released three times, and returns its
// directory. The tags carry the "alpha/" prefix a module below the root of its
// repository is released under.
func chainRepo(t *testing.T) string {
	t.Helper()

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")

	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Greet greets.\nfunc Greet(name string) string { return name }\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: add Greet")
	runGit(t, root, "tag", "alpha/v0.0.1")

	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Bye parts.\nfunc Bye(name string) string { return name }\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: replace Greet with Bye")
	runGit(t, root, "tag", "alpha/v0.1.0")

	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Bye parts.\nfunc Bye(name string) string { return name }\n\n// Hi greets.\nfunc Hi() string { return \"hi\" }\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: add Hi")
	runGit(t, root, "tag", "alpha/v0.1.1")

	return alpha
}

func TestReadVerdictsCoversEveryRelease(t *testing.T) {
	requireGoFsck(t)

	alpha := chainRepo(t)
	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Bye parts.\nfunc Bye(name string) string { return name }\n\n// Hi greets.\nfunc Hi() string { return \"hi\" }\n\n// Extra is pending.\nfunc Extra() {}\n")
	runGit(t, filepath.Dir(alpha), "commit", "--quiet", "-am", "alpha: add Extra")

	got, err := readVerdicts(alpha, false, "", "")
	if err != nil {
		t.Fatalf("readVerdicts() error: %v", err)
	}

	// Newest first: the pending release, the latest release against the opener
	// of its series, then the series bump and the first tag.
	want := []struct {
		version  string
		since    string
		released bool
	}{
		{version: "v0.1.2", since: "v0.1.1"},
		{version: "v0.1.1", since: "v0.1.0", released: true},
		{version: "v0.1.0", since: "v0.0.1", released: true},
		{version: "v0.0.1", released: true},
	}
	if len(got) != len(want) {
		t.Fatalf("readVerdicts() = %d sections, want %d: %+v", len(got), len(want), chainVersions(got))
	}
	for i, w := range want {
		if got[i].Version != w.version || got[i].Since != w.since || got[i].Released != w.released {
			t.Errorf("section %d = {Version: %q, Since: %q, Released: %v}, want {%q, %q, %v}",
				i, got[i].Version, got[i].Since, got[i].Released, w.version, w.since, w.released)
		}
	}

	// The first section has the whole history behind it, and everything the
	// release exports is an addition, since there is no earlier release for it
	// to be measured against.
	first := got[len(got)-1]
	if len(first.Commits) == 0 {
		t.Error("the first release reported no commits, want the history behind it")
	}
	if first.API.Skipped != "" {
		t.Fatalf("the first release API.Skipped = %q, want the API read against nothing", first.API.Skipped)
	}
	if len(first.API.Added) == 0 || first.API.Breaking {
		t.Errorf("the first release API = %#v, want everything it exports added and nothing breaking", first.API)
	}

	// Every section covers the module only, and names it.
	for i, v := range got {
		if v.Module != "example.com/alpha" {
			t.Errorf("section %d Module = %q, want example.com/alpha", i, v.Module)
		}
	}
}

// chainVersions names the release each section reports on, for a failure
// message.
func chainVersions(verdicts []verdict) []string {
	out := make([]string, 0, len(verdicts))
	for _, v := range verdicts {
		out = append(out, v.Version)
	}
	return out
}

func TestReadVerdictsComparesTheAPIOfEveryRelease(t *testing.T) {
	requireGoFsck(t)

	got, err := readVerdicts(chainRepo(t), false, "", "")
	if err != nil {
		t.Fatalf("readVerdicts() error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("readVerdicts() = %d sections, want 3: %v", len(got), chainVersions(got))
	}

	// v0.1.1 against the opener of its series, which added Hi.
	if len(got[0].API.Added) != 1 || got[0].API.Added[0].Name != "Hi" {
		t.Errorf("v0.1.1 API.Added = %#v, want Hi", got[0].API.Added)
	}
	// v0.1.0 against v0.0.1, which replaced Greet with Bye.
	if len(got[1].API.Removed) != 1 || got[1].API.Removed[0].Name != "Greet" {
		t.Errorf("v0.1.0 API.Removed = %#v, want Greet", got[1].API.Removed)
	}
	if !got[1].API.Breaking {
		t.Error("v0.1.0 removed an exported symbol but was not reported as breaking")
	}
}

func TestReadVerdictsVerboseReportsEveryTag(t *testing.T) {
	got, err := readVerdicts(chainRepo(t), true, "", "")
	if err != nil {
		t.Fatalf("readVerdicts() error: %v", err)
	}
	if want := []string{"v0.1.1", "v0.1.0", "v0.0.1"}; strings.Join(chainVersions(got), " ") != strings.Join(want, " ") {
		t.Errorf("readVerdicts(verbose) = %v, want %v", chainVersions(got), want)
	}
}

func TestReadVerdictsBoundedByTo(t *testing.T) {
	got, err := readVerdicts(chainRepo(t), false, "", "v0.1.0")
	if err != nil {
		t.Fatalf("readVerdicts() error: %v", err)
	}
	if want := []string{"v0.1.0", "v0.0.1"}; strings.Join(chainVersions(got), " ") != strings.Join(want, " ") {
		t.Errorf("readVerdicts(upTo v0.1.0) = %v, want %v", chainVersions(got), want)
	}
}

func TestReadVerdictsBoundedByFrom(t *testing.T) {
	got, err := readVerdicts(chainRepo(t), false, "v0.1.0", "")
	if err != nil {
		t.Fatalf("readVerdicts() error: %v", err)
	}
	// The chain starts at v0.1.0, so the first release is out of it.
	if want := []string{"v0.1.1"}; strings.Join(chainVersions(got), " ") != strings.Join(want, " ") {
		t.Errorf("readVerdicts(from v0.1.0) = %v, want %v", chainVersions(got), want)
	}
	if got[0].Since != "v0.1.0" {
		t.Errorf("readVerdicts(from v0.1.0) Since = %q, want v0.1.0", got[0].Since)
	}
}

func TestReadVerdictsWithNoReleaseInTheRange(t *testing.T) {
	if _, err := readVerdicts(chainRepo(t), false, "v9.0.0", ""); err == nil {
		t.Error("readVerdicts() reported on a range holding no release")
	}
}

func TestReadVerdictsWithoutAReleaseTag(t *testing.T) {
	root := testRepo(t, "alpha")

	got, err := readVerdicts(filepath.Join(root, "alpha"), false, "", "")
	if err != nil {
		t.Fatalf("readVerdicts() error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("readVerdicts() = %d sections, want 1: %v", len(got), chainVersions(got))
	}
	if got[0].Released || got[0].Version != "v0.0.1" || got[0].Since != "" {
		t.Errorf("readVerdicts() = {Released: %v, Version: %q, Since: %q}, want {false, v0.0.1, \"\"}",
			got[0].Released, got[0].Version, got[0].Since)
	}
}

func TestRenderVerdictsWritesEverySection(t *testing.T) {
	first := sampleVerdict()
	second := sampleVerdict()
	second.Version, second.Since = "v1.0.0", "v0.9.0"

	var out bytes.Buffer
	renderVerdicts(&out, []verdict{first, second}, false)

	got := out.String()
	for _, want := range []string{"# example.com/x @ v1.1.0", "# example.com/x @ v1.0.0"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVerdicts() output missing %q:\n%s", want, got)
		}
	}
	if strings.Index(got, "v1.1.0") > strings.Index(got, "# example.com/x @ v1.0.0") {
		t.Error("renderVerdicts() wrote the sections out of order, want newest first")
	}

	// One section renders exactly as it does on its own, so a chain of one is
	// the report the single-range command writes.
	var single bytes.Buffer
	renderVerdict(&single, first, false)
	var chain bytes.Buffer
	renderVerdicts(&chain, []verdict{first}, false)
	if chain.String() != single.String() {
		t.Errorf("renderVerdicts() of one section = %q, want the single verdict %q", chain.String(), single.String())
	}
}

func TestParseOptionsVerdictChain(t *testing.T) {
	tests := []struct {
		args  []string
		chain bool
		from  string
	}{
		{args: []string{"worktree", "verdict", "--all"}, chain: true},
		{args: []string{"worktree", "verdict", "--from=all"}, chain: true},
		{args: []string{"worktree", "verdict", "--from=0"}, chain: true},
		{args: []string{"worktree", "verdict", "--from=HEAD"}, chain: true},
		{args: []string{"worktree", "verdict", "--from=v0.0.0"}, chain: true},
		{args: []string{"worktree", "verdict", "--from", "0"}, chain: true},
		{args: []string{"worktree", "verdict", "--from", "all", "platform"}, chain: true},
		// A revision names one range, as it always did.
		{args: []string{"worktree", "verdict", "--from=v0.1.0"}, from: "v0.1.0"},
		{args: []string{"worktree", "verdict", "--from=main"}, from: "main"},
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
			if !opts.Verdict {
				t.Fatalf("%v: Verdict = false, want true", test.args)
			}
			if opts.Chain != test.chain {
				t.Errorf("%v: Chain = %v, want %v", test.args, opts.Chain, test.chain)
			}
			if opts.From != test.from {
				t.Errorf("%v: From = %q, want %q", test.args, opts.From, test.from)
			}
		}()
	}
}

func TestChainKeyword(t *testing.T) {
	tests := map[string]bool{
		"all": true, "ALL": true, " all ": true,
		"0": true, "0.0.0": true, "v0.0.0": true,
		"HEAD": true, "head": true,
		"": false, "v0.1.0": false, "main": false, "0.1": false, "HEAD~1": false,
	}

	for from, want := range tests {
		if got := chainKeyword(from); got != want {
			t.Errorf("chainKeyword(%q) = %v, want %v", from, got, want)
		}
	}
}

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
