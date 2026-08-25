package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestBrowsableRemote(t *testing.T) {
	tests := map[string]string{
		"git@github.com:titpetric/platform.git":       "https://github.com/titpetric/platform",
		"git@github.com:titpetric/platform":           "https://github.com/titpetric/platform",
		"ssh://git@github.com/titpetric/platform.git": "https://github.com/titpetric/platform",
		"https://github.com/titpetric/platform.git":   "https://github.com/titpetric/platform",
		"https://github.com/titpetric/platform":       "https://github.com/titpetric/platform",
		"http://git.example.com/team/project.git":     "http://git.example.com/team/project",
		"git@git.example.com:2222/team/project.git":   "https://git.example.com/2222/team/project",
		"  git@github.com:titpetric/platform.git\n":   "https://github.com/titpetric/platform",
		// Nothing a browser opens, so nothing to link to.
		"":                        "",
		"/srv/git/project.git":    "",
		"git://github.com/x/y":    "",
		"file:///srv/git/project": "",
	}

	for remote, want := range tests {
		if got := browsableRemote(remote); got != want {
			t.Errorf("browsableRemote(%q) = %q, want %q", remote, got, want)
		}
	}
}

func TestCommitLogCoversTheModuleOnly(t *testing.T) {
	root := testRepo(t, "alpha", "beta")
	runGit(t, root, "tag", "alpha/v0.1.0")

	writeTestFile(t, filepath.Join(root, "alpha", "alpha.go"), "package alpha\n\n// changed\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: change")
	writeTestFile(t, filepath.Join(root, "beta", "beta.go"), "package beta\n\n// changed\n")
	runGit(t, root, "commit", "--quiet", "-am", "beta: change")
	runGit(t, root, "tag", "alpha/v0.2.0")

	alpha := filepath.Join(root, "alpha")

	got := commitLogSinceTag(alpha, "alpha/v0.1.0")
	if len(got) != 1 || got[0].Subject != "alpha: change" {
		t.Fatalf("commitLogSinceTag() = %#v, want the one commit touching alpha", got)
	}
	if got[0].Hash == "" {
		t.Error("commitLogSinceTag() returned a commit without a hash")
	}

	// A range between two tags is what a released module reports on.
	if got := commitLogBetween(alpha, "alpha/v0.1.0", "alpha/v0.2.0"); len(got) != 1 {
		t.Errorf("commitLogBetween() = %d commits, want 1", len(got))
	}

	// Without a tag the whole history of the module is what a first release
	// would cover.
	if got := commitLogSinceTag(alpha, ""); len(got) != 2 {
		t.Errorf("commitLogSinceTag() without a tag = %d commits, want 2", len(got))
	}
}

// verdictRepo builds a module with two releases and returns its directory. The
// second release removes Greet and adds Bye, so it is a breaking one.
func verdictRepo(t *testing.T) string {
	t.Helper()

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")

	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Greet greets.\nfunc Greet(name string) string { return name }\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: add Greet")
	runGit(t, root, "tag", "alpha/v0.1.0")

	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Bye parts.\nfunc Bye(name string) string { return name }\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: replace Greet with Bye")
	runGit(t, root, "tag", "alpha/v0.2.0")

	return alpha
}

func TestReadVerdictReportsTheLastReleaseWhenLevelWithItsTag(t *testing.T) {
	requireGoFsck(t)

	got, err := readVerdict(verdictRepo(t), "", "")
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}

	if !got.Released {
		t.Error("readVerdict() proposed a release for a module level with its tag")
	}
	if got.Version != "v0.2.0" || got.Since != "v0.1.0" {
		t.Errorf("readVerdict() = {Version: %q, Since: %q}, want {v0.2.0, v0.1.0}", got.Version, got.Since)
	}
	if len(got.Commits) != 1 || got.Commits[0].Subject != "alpha: replace Greet with Bye" {
		t.Errorf("readVerdict() Commits = %#v, want the commit between the two tags", got.Commits)
	}
	// The comparison is between the tags, not against the working tree.
	if len(got.API.Removed) != 1 || got.API.Removed[0].Name != "Greet" {
		t.Errorf("readVerdict() API.Removed = %#v, want Greet", got.API.Removed)
	}
	if want := "Released v0.2.0: 1 exported symbol was removed since v0.1.0."; got.Summary() != want {
		t.Errorf("Summary() = %q, want %q", got.Summary(), want)
	}
}

func TestReadVerdictProposesAReleaseWhenBehind(t *testing.T) {
	requireGoFsck(t)

	alpha := verdictRepo(t)
	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Bye parts.\nfunc Bye(name string) string { return name }\n\n// Hi greets.\nfunc Hi() string { return \"hi\" }\n")
	runGit(t, filepath.Dir(alpha), "commit", "--quiet", "-am", "alpha: add Hi")

	got, err := readVerdict(alpha, "", "")
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}

	if got.Released {
		t.Error("readVerdict() reported a release that has not been made")
	}
	if got.Version != "v0.2.1" || got.Since != "v0.2.0" || got.Release != releasePatch {
		t.Errorf("readVerdict() = {Version: %q, Since: %q, Release: %q}, want {v0.2.1, v0.2.0, patch}", got.Version, got.Since, got.Release)
	}
	if len(got.API.Added) != 1 || got.API.Added[0].Name != "Hi" {
		t.Errorf("readVerdict() API.Added = %#v, want Hi", got.API.Added)
	}
}

func TestReadVerdictWithOneTagHasNothingToCompare(t *testing.T) {
	root := testRepo(t, "alpha")
	runGit(t, root, "tag", "alpha/v0.1.0")

	got, err := readVerdict(filepath.Join(root, "alpha"), "", "")
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if !got.Released || got.Version != "v0.1.0" {
		t.Errorf("readVerdict() = {Released: %v, Version: %q}, want {true, v0.1.0}", got.Released, got.Version)
	}
	if !strings.Contains(got.API.Skipped, "no earlier release") {
		t.Errorf("readVerdict() API.Skipped = %q, want it to say there is no earlier release", got.API.Skipped)
	}
}

func TestReadVerdictWithoutAReleaseTag(t *testing.T) {
	root := testRepo(t, "alpha")

	got, err := readVerdict(filepath.Join(root, "alpha"), "", "")
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if got.Released || got.Version != "v0.0.1" || got.Since != "" {
		t.Errorf("readVerdict() = {Released: %v, Version: %q, Since: %q}, want {false, v0.0.1, \"\"}", got.Released, got.Version, got.Since)
	}
	if len(got.Commits) == 0 {
		t.Error("readVerdict() without a tag reported no commits, want the whole history")
	}
}

func TestApiDiffBetweenReadsTwoTagsAndNotTheWorkingTree(t *testing.T) {
	requireGoFsck(t)

	alpha := verdictRepo(t)
	// A working tree that would swamp the answer if it were being read.
	writeTestFile(t, filepath.Join(alpha, "stray.go"), "package alpha\n\n// Stray is uncommitted.\nfunc Stray() {}\n")

	got := apiDiffBetween(alpha, "alpha/v0.1.0", "alpha/v0.2.0", false)
	if got.Skipped != "" {
		t.Fatalf("apiDiffBetween() skipped: %s", got.Skipped)
	}
	if len(got.Added) != 1 || got.Added[0].Name != "Bye" {
		t.Errorf("apiDiffBetween() Added = %#v, want Bye alone", got.Added)
	}
	if !got.Breaking || len(got.Removed) != 1 {
		t.Errorf("apiDiffBetween() = %#v, want a breaking result removing Greet", got)
	}
}

func TestApiDiffBetweenReadsTypeBodiesWithSources(t *testing.T) {
	requireGoFsck(t)

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")
	runGit(t, root, "tag", "alpha/v0.1.0")

	// Grouped field names are the case that reading the body from the source
	// rather than from the decomposed fields is for.
	writeTestFile(t, filepath.Join(alpha, "alpha.go"),
		"package alpha\n\n// Tag is a release tag.\ntype Tag struct {\n\tName string\n\n\tMajor, Minor, Patch uint64\n}\n")

	got := apiDiffBetween(alpha, "alpha/v0.1.0", "", true)
	if got.Skipped != "" {
		t.Fatalf("apiDiffBetween() skipped: %s", got.Skipped)
	}
	if len(got.Added) != 1 {
		t.Fatalf("apiDiffBetween() Added = %#v, want Tag alone", got.Added)
	}

	body := got.Added[0].Definition
	if strings.Contains(body, "// Tag is a release tag.") {
		t.Errorf("the doc comment was kept:\n%s", body)
	}
	// The body is printed as go-fsck formats it, which aligns fields with
	// tabs, so the names are what is asserted rather than the spacing.
	for _, name := range []string{"Name", "Major, Minor, Patch"} {
		if !strings.Contains(body, name) {
			t.Errorf("the body lost %q:\n%s", name, body)
		}
	}
	if !strings.HasPrefix(body, "type Tag struct {") {
		t.Errorf("the body does not open on the declaration:\n%s", body)
	}

	// Without sources there is no body to print.
	if plain := apiDiffBetween(alpha, "alpha/v0.1.0", "", false); plain.Added[0].Definition != "" {
		t.Errorf("apiDiffBetween() without sources carried a body: %q", plain.Added[0].Definition)
	}
}

func TestVerdictSummary(t *testing.T) {
	tests := []struct {
		name string
		in   verdict
		want string
	}{
		{
			name: "first release",
			in:   verdict{Version: "v0.0.1"},
			want: "First release: v0.0.1, with no earlier release to compare against.",
		},
		{
			name: "patch",
			in:   verdict{Version: "v1.0.1", Since: "v1.0.0", Release: releasePatch},
			want: "Patch release: v1.0.1, no exported symbols were removed since v1.0.0.",
		},
		{
			name: "minor, one removal",
			in: verdict{
				Version: "v1.1.0", Since: "v1.0.0", Release: releaseMinor,
				API: apiDiff{Removed: []apiSymbol{{Key: "x.A"}}, Breaking: true},
			},
			want: "Minor release: v1.1.0, because 1 exported symbol was removed since v1.0.0.",
		},
		{
			name: "minor, removals and signature changes",
			in: verdict{
				Version: "v1.1.0", Since: "v1.0.0", Release: releaseMinor,
				API: apiDiff{
					Removed:  []apiSymbol{{Key: "x.A"}, {Key: "x.B"}},
					Changed:  []apiChange{{Key: "x.C"}},
					Breaking: true,
				},
			},
			want: "Minor release: v1.1.0, because 2 exported symbols were removed and 1 signature changed since v1.0.0.",
		},
		{
			name: "nothing to compare",
			in: verdict{
				Version: "v1.0.1", Since: "v1.0.0", Release: releasePatch,
				API: apiDiff{Skipped: "go-fsck is not installed"},
			},
			want: "Patch release: v1.0.1, the API was not compared, go-fsck is not installed.",
		},
		{
			name: "a release that was made",
			in:   verdict{Version: "v1.1.0", Since: "v1.0.0", Released: true},
			want: "Released v1.1.0: no exported symbols were removed since v1.0.0.",
		},
		{
			name: "a release that was made, breaking",
			in: verdict{
				Version: "v1.1.0", Since: "v1.0.0", Released: true,
				API: apiDiff{Removed: []apiSymbol{{Key: "x.A"}}, Breaking: true},
			},
			want: "Released v1.1.0: 1 exported symbol was removed since v1.0.0.",
		},
		{
			name: "a release with nothing before it",
			in: verdict{
				Version: "v1.0.0", Released: true,
				API: apiDiff{Skipped: "no earlier release to compare against"},
			},
			want: "Released v1.0.0, no earlier release to compare against.",
		},
	}

	for _, test := range tests {
		if got := test.in.Summary(); got != test.want {
			t.Errorf("%s: Summary() = %q, want %q", test.name, got, test.want)
		}
	}
}

// sampleVerdict is a verdict holding one of everything the report can show.
func sampleVerdict() verdict {
	return verdict{
		Module:  "example.com/x",
		Version: "v1.1.0",
		Since:   "v1.0.0",
		Release: releaseMinor,
		RepoURL: "https://github.com/example/x",
		Commits: []commitLog{
			{Hash: "abc1234", Subject: "feat: add Client"},
			{Hash: "def5678", Subject: "refactor: drop Legacy"},
		},
		API: apiDiff{
			Removed: []apiSymbol{{
				Key: "example.com/x.Legacy", Package: "example.com/x",
				Name: "Legacy", Kind: "func", Signature: "func Legacy () error",
			}},
			Added: []apiSymbol{{
				Key: "example.com/x.Client", Package: "example.com/x",
				Name: "Client", Kind: "type",
				Definition: "type Client struct {\n\tName string\n}",
			}},
			Changed: []apiChange{{
				Key: "example.com/x.Open", Package: "example.com/x",
				Name: "Open", Old: "Open ()", New: "Open (string)",
			}},
			Breaking: true,
		},
	}
}

func TestRenderVerdictMarkdown(t *testing.T) {
	var out bytes.Buffer
	renderVerdict(&out, sampleVerdict(), false)

	got := out.String()
	for _, want := range []string{
		"# example.com/x v1.1.0",
		"Minor release: v1.1.0, because 1 exported symbol was removed and 1 signature changed since v1.0.0.",
		"## Commits since v1.0.0",
		"| [`abc1234`](https://github.com/example/x/commit/abc1234) | feat: add Client |",
		"## API since v1.0.0",
		"| Change | Symbol |",
		"| Removed | func Legacy () error |",
		"| Changed | Open () -> Open (string) |",
		"| Added | type Client |",
		"<details>\n<summary><code>type Client</code></summary>",
		"```go\ntype Client struct {\n\tName string\n}\n```",
		"</details>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
		}
	}

	if strings.Contains(got, "\033") {
		t.Error("renderVerdict() wrote escape codes into markdown")
	}
	// A module with one package has nothing to disambiguate.
	if strings.Contains(got, "| Package |") {
		t.Errorf("renderVerdict() added a package column for a single package:\n%s", got)
	}
}

func TestRenderVerdictANSI(t *testing.T) {
	var out bytes.Buffer
	renderVerdict(&out, sampleVerdict(), true)

	got := out.String()
	if !strings.Contains(got, "\033") {
		t.Error("renderVerdict() wrote no escape codes to a terminal")
	}
	for _, unwanted := range []string{"<details>", "<summary>", "```", "# example.com/x", "| Change |"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("renderVerdict() wrote markup %q to a terminal:\n%s", unwanted, got)
		}
	}

	plain := ansi.Strip(got)
	for _, want := range []string{
		"example.com/x v1.1.0",
		"Commits since v1.0.0",
		"abc1234",
		"Removed",
		"func Legacy () error",
		"type Client struct {",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, plain)
		}
	}
	if !strings.Contains(plain, "╭") {
		t.Errorf("renderVerdict() drew no table:\n%s", plain)
	}
}

func TestRenderVerdictNamesThePackageWhenSymbolsSpanMoreThanOne(t *testing.T) {
	v := sampleVerdict()
	v.API.Added = append(v.API.Added, apiSymbol{
		Key: "example.com/x/inner.Name", Package: "example.com/x/inner", Name: "Name", Kind: "const",
	})

	var out bytes.Buffer
	renderVerdict(&out, v, false)

	got := out.String()
	for _, want := range []string{"| Change | Package | Symbol |", "| inner | const Name |", "|  | type Client |"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderVerdictWithoutARemoteLeavesHashesUnlinked(t *testing.T) {
	v := sampleVerdict()
	v.RepoURL = ""

	var out bytes.Buffer
	renderVerdict(&out, v, false)

	got := out.String()
	if !strings.Contains(got, "| `abc1234` | feat: add Client |") {
		t.Errorf("renderVerdict() did not fall back to a plain hash:\n%s", got)
	}
	if strings.Contains(got, "](") {
		t.Errorf("renderVerdict() linked a hash without a remote to link into:\n%s", got)
	}
}

func TestRenderVerdictOmitsEmptySections(t *testing.T) {
	var out bytes.Buffer
	renderVerdict(&out, verdict{Module: "example.com/x", Version: "v1.0.1", Since: "v1.0.0", Release: releasePatch}, false)

	got := out.String()
	if strings.Contains(got, "##") {
		t.Errorf("renderVerdict() wrote a heading with nothing under it:\n%s", got)
	}
	if !strings.Contains(got, "Patch release: v1.0.1") {
		t.Errorf("renderVerdict() did not report the release:\n%s", got)
	}
}

func TestParseOptionsVerdict(t *testing.T) {
	tests := []struct {
		args   []string
		filter string
	}{
		{args: []string{"worktree", "verdict"}},
		{args: []string{"worktree", "verdict", "platform"}, filter: "platform"},
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
			if opts.FilterArg != test.filter {
				t.Errorf("%v: FilterArg = %q, want %q", test.args, opts.FilterArg, test.filter)
			}
		}()
	}
}

func TestGoSeriesChanged(t *testing.T) {
	tests := []struct {
		before, after string
		want          bool
	}{
		// Another release series is what a consumer feels.
		{"1.26", "1.27", true},
		{"1.26.4", "1.27.0", true},
		{"go1.26", "go1.27", true},
		{"1.27", "1.26", true},
		// A point release of the same series changes nothing for them.
		{"1.27", "1.27", false},
		{"1.27", "1.27.1", false},
		{"1.27.1", "1.27.9", false},
		// Nothing to read on either side is not a change.
		{"", "1.27", false},
		{"1.27", "", false},
		{"nonsense", "1.27", false},
	}

	for _, test := range tests {
		if got := goSeriesChanged(test.before, test.after); got != test.want {
			t.Errorf("goSeriesChanged(%q, %q) = %v, want %v", test.before, test.after, got, test.want)
		}
	}
}

func TestVerdictMovingGoSeriesCostsAMinor(t *testing.T) {
	requireGoFsck(t)

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")
	runGit(t, root, "tag", "alpha/v0.1.0")

	// Nothing of the API moves, only the language version.
	writeTestFile(t, filepath.Join(alpha, "go.mod"), "module example.com/alpha\n\ngo 1.27\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: move to go 1.27")

	got, err := readVerdict(alpha, "", "")
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if got.GoBefore != "1.24" || got.GoAfter != "1.27" {
		t.Errorf("readVerdict() = {GoBefore: %q, GoAfter: %q}, want {1.24, 1.27}", got.GoBefore, got.GoAfter)
	}
	if got.Release != releaseMinor || got.Version != "v0.2.0" {
		t.Errorf("readVerdict() = {Release: %q, Version: %q}, want {minor, v0.2.0}", got.Release, got.Version)
	}
	if want := "Minor release: v0.2.0, because go moved from 1.24 to 1.27 since v0.1.0."; got.Summary() != want {
		t.Errorf("Summary() = %q, want %q", got.Summary(), want)
	}

	// Once that release is made, a point release of the same series is not
	// worth another minor.
	runGit(t, root, "tag", "alpha/v0.2.0")
	writeTestFile(t, filepath.Join(alpha, "go.mod"), "module example.com/alpha\n\ngo 1.27.3\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: move to go 1.27.3")

	got, err = readVerdict(alpha, "", "")
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if got.Release != releasePatch || got.Version != "v0.2.1" {
		t.Errorf("readVerdict() = {Release: %q, Version: %q} for a point release, want {patch, v0.2.1}", got.Release, got.Version)
	}
}

func TestReadVerdictBetweenNamedRevisions(t *testing.T) {
	requireGoFsck(t)

	alpha := verdictRepo(t)
	// A working tree the report must not read, since a range was named.
	writeTestFile(t, filepath.Join(alpha, "stray.go"), "package alpha\n\n// Stray is uncommitted.\nfunc Stray() {}\n")

	got, err := readVerdict(alpha, "v0.1.0", "v0.2.0")
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if !got.Released || got.Version != "v0.2.0" || got.Since != "v0.1.0" {
		t.Errorf("readVerdict() = {Released: %v, Version: %q, Since: %q}, want {true, v0.2.0, v0.1.0}", got.Released, got.Version, got.Since)
	}
	if len(got.API.Removed) != 1 || got.API.Removed[0].Name != "Greet" {
		t.Errorf("readVerdict() API.Removed = %#v, want Greet", got.API.Removed)
	}
	for _, symbol := range got.API.Added {
		if symbol.Name == "Stray" {
			t.Error("readVerdict() read the working tree for a named range")
		}
	}

	// Naming only the newer revision measures from the release below it.
	got, err = readVerdict(alpha, "", "v0.2.0")
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if got.Since != "v0.1.0" {
		t.Errorf("readVerdict(--to) Since = %q, want v0.1.0", got.Since)
	}

	// Naming only the older one measures to the working tree, where Stray is.
	got, err = readVerdict(alpha, "v0.1.0", "")
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if got.Released {
		t.Error("readVerdict(--from) reported a release that has not been made")
	}
	var found bool
	for _, symbol := range got.API.Added {
		found = found || symbol.Name == "Stray"
	}
	if !found {
		t.Errorf("readVerdict(--from) did not read the working tree: %#v", got.API.Added)
	}
}

func TestTaggedRef(t *testing.T) {
	tests := []struct {
		ref, prefix, want string
	}{
		{"v0.1.0", "alpha/", "alpha/v0.1.0"},
		{"v0.1.0", "", "v0.1.0"},
		// Anything that is not a version is a commit or a branch, and is left
		// as it was given.
		{"HEAD", "alpha/", "HEAD"},
		{"main", "alpha/", "main"},
		{"29097b5", "alpha/", "29097b5"},
		{"", "alpha/", ""},
	}

	for _, test := range tests {
		if got := taggedRef(test.ref, test.prefix); got != test.want {
			t.Errorf("taggedRef(%q, %q) = %q, want %q", test.ref, test.prefix, got, test.want)
		}
	}
}

func TestParseOptionsVerdictRange(t *testing.T) {
	args, commandLine := os.Args, flag.CommandLine
	t.Cleanup(func() { os.Args, flag.CommandLine = args, commandLine })

	os.Args = []string{"worktree", "verdict", "--from", "v0.1.0", "--to=v0.2.0"}
	flag.CommandLine = flag.NewFlagSet("worktree", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	opts := ParseOptions()
	if !opts.Verdict {
		t.Fatal("Verdict = false, want true")
	}
	if opts.From != "v0.1.0" || opts.To != "v0.2.0" {
		t.Errorf("= {From: %q, To: %q}, want {v0.1.0, v0.2.0}", opts.From, opts.To)
	}
}
