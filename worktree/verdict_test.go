package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

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
	requireSplint(t)

	got, err := readVerdict(verdictRepo(t), "", "", false)
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
	requireSplint(t)

	alpha := verdictRepo(t)
	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Bye parts.\nfunc Bye(name string) string { return name }\n\n// Hi greets.\nfunc Hi() string { return \"hi\" }\n")
	runGit(t, filepath.Dir(alpha), "commit", "--quiet", "-am", "alpha: add Hi")

	got, err := readVerdict(alpha, "", "", false)
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

// The first release has no earlier one to be compared against, so everything it
// exports is reported as added.
func TestReadVerdictWithOneTagAddsEverythingItExports(t *testing.T) {
	requireSplint(t)

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")
	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Greet greets.\nfunc Greet(name string) string { return name }\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: add Greet")
	runGit(t, root, "tag", "alpha/v0.1.0")

	got, err := readVerdict(alpha, "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if !got.Released || got.Version != "v0.1.0" || got.Since != "" {
		t.Errorf("readVerdict() = {Released: %v, Version: %q, Since: %q}, want {true, v0.1.0, \"\"}", got.Released, got.Version, got.Since)
	}
	if got.API.Skipped != "" {
		t.Fatalf("readVerdict() API.Skipped = %q, want the API read against nothing", got.API.Skipped)
	}
	if len(got.API.Added) != 1 || got.API.Added[0].Name != "Greet" {
		t.Errorf("readVerdict() API.Added = %#v, want Greet", got.API.Added)
	}
	if got.API.Breaking {
		t.Error("readVerdict() called a first release breaking")
	}
}

func TestReadVerdictWithoutAReleaseTag(t *testing.T) {
	requireSplint(t)

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")
	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Greet greets.\nfunc Greet(name string) string { return name }\n\n// Tag is a release tag.\ntype Tag struct {\n\tName string\n}\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: add Greet")

	got, err := readVerdict(alpha, "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if got.Released || got.Version != "v0.0.1" || got.Since != "" {
		t.Errorf("readVerdict() = {Released: %v, Version: %q, Since: %q}, want {false, v0.0.1, \"\"}", got.Released, got.Version, got.Since)
	}
	if len(got.Commits) == 0 {
		t.Error("readVerdict() without a tag reported no commits, want the whole history")
	}

	// Everything the working tree exports is an addition, since the release is
	// measured against a module holding nothing at all.
	var names []string
	for _, symbol := range got.API.Added {
		names = append(names, symbol.Name)
	}
	if want := []string{"Greet", "Tag"}; !reflect.DeepEqual(names, want) {
		t.Errorf("readVerdict() API.Added = %v, want %v", names, want)
	}
	if got.Release != releasePatch {
		t.Errorf("readVerdict() Release = %q, want a patch: a first release takes nothing away", got.Release)
	}

	// The fields of an added type are read as additions of their own, so the
	// first release states the shape it declares.
	if fields := len(dataModelEntries(got)); fields != 1 {
		t.Errorf("dataModelEntries() = %d entries, want the one field of Tag", fields)
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
			{Hash: "abc1234", Subject: "feat: add Client", Published: true},
			{Hash: "def5678", Subject: "refactor: drop Legacy", Published: true},
		},
		API: apiDiff{
			Removed: []apiSymbol{{
				Key: "example.com/x.Legacy", Package: "example.com/x",
				Name: "Legacy", Kind: "func", Exported: true, Signature: "func Legacy () error",
			}},
			Added: []apiSymbol{{
				Key: "example.com/x.Client", Package: "example.com/x",
				Name: "Client", Kind: "type", Exported: true, Underlying: "struct",
				Fields: []apiField{{Name: "Name", Type: "string", Tag: `json:"name"`}},
			}},
			Changed: []apiChange{{
				Key: "example.com/x.Open", Package: "example.com/x",
				Name: "Open", Exported: true, Old: "Open ()", New: "Open (string)",
			}},
			Types: []apiTypeChange{{
				Key: "example.com/x.Config", Package: "example.com/x",
				Name: "Config", Underlying: "struct", Breaking: true,
				Fields: []apiFieldChange{
					{
						Name: "Addr", Change: fieldChanged,
						Old: &apiField{Name: "Addr", Type: "string", Tag: `yaml:"addr"`},
						New: &apiField{Name: "Addr", Type: "[]string", Tag: `yaml:"addr"`},
					},
					{Name: "Timeout", Change: fieldAdded, New: &apiField{Name: "Timeout", Type: "int"}},
					{Name: "Retries", Change: fieldRemoved, Old: &apiField{Name: "Retries", Type: "int"}},
				},
			}},
			Breaking: true,
		},
		Visibility: visibilityReport{Packages: []visibilityPackage{
			{Package: "./", ExportedTypes: 9, InternalTypes: 1, ExportedFuncs: 43, InternalFuncs: 37, InternalRatio: 44.8},
			{Package: "./storage", ExportedTypes: 3, InternalTypes: 1, ExportedFuncs: 22, InternalFuncs: 3, InternalRatio: 8.4},
		}},
	}
}

// sampleInterfaceChange is a verdict whose data model change is to an
// interface, whose fields read as a method set.
func sampleInterfaceChange() verdict {
	v := sampleVerdict()
	v.API.Added = nil
	v.API.Types = []apiTypeChange{{
		Key: "example.com/x/store.Store", Package: "example.com/x/store",
		Name: "Store", Underlying: "interface", Breaking: true,
		Fields: []apiFieldChange{
			{Name: "Put", Change: fieldAdded, New: &apiField{Name: "Put", Type: "Put (key string) error"}},
		},
	}}
	return v
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

func TestReadVerdictBetweenNamedRevisions(t *testing.T) {
	requireSplint(t)

	alpha := verdictRepo(t)
	// A working tree the report must not read, since a range was named.
	writeTestFile(t, filepath.Join(alpha, "stray.go"), "package alpha\n\n// Stray is uncommitted.\nfunc Stray() {}\n")

	got, err := readVerdict(alpha, "v0.1.0", "v0.2.0", false)
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
	got, err = readVerdict(alpha, "", "v0.2.0", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if got.Since != "v0.1.0" {
		t.Errorf("readVerdict(--to) Since = %q, want v0.1.0", got.Since)
	}

	// Naming only the older one measures to the working tree, where Stray is.
	got, err = readVerdict(alpha, "v0.1.0", "", false)
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
	tags := []string{"v0.1.0", "v0.2.0"}
	tests := []struct {
		ref, prefix, want string
		tags              []string
	}{
		{ref: "v0.1.0", prefix: "alpha/", tags: tags, want: "alpha/v0.1.0"},
		{ref: "v0.1.0", tags: tags, want: "v0.1.0"},
		// The version is matched against the tags rather than reprinted, so the
		// v the command line left off is the one the repository carries.
		{ref: "0.1.0", tags: tags, want: "v0.1.0"},
		{ref: "0.1.0", prefix: "alpha/", tags: tags, want: "alpha/v0.1.0"},
		// A repository tagging without the v is found from either spelling.
		{ref: "v0.3.0", tags: []string{"0.3.0"}, want: "0.3.0"},
		// A version no tag carries is passed to git as it stands, so the run
		// reports an unreadable revision rather than falling back elsewhere.
		{ref: "v9.9.9", prefix: "alpha/", tags: tags, want: "alpha/v9.9.9"},
		{ref: "v0.1.0", prefix: "alpha/", want: "alpha/v0.1.0"},
		// Anything that is not a version is a commit or a branch, and is left
		// as it was given.
		{ref: "HEAD", prefix: "alpha/", tags: tags, want: "HEAD"},
		{ref: "main", prefix: "alpha/", tags: tags, want: "main"},
		{ref: "29097b5", prefix: "alpha/", tags: tags, want: "29097b5"},
		{ref: "", prefix: "alpha/", tags: tags, want: ""},
	}

	for _, test := range tests {
		if got := taggedRef(test.ref, test.prefix, test.tags); got != test.want {
			t.Errorf("taggedRef(%q, %q, %v) = %q, want %q", test.ref, test.prefix, test.tags, got, test.want)
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

// commitScanRepo builds a module whose history holds one commit of each kind
// the commit table has a cell for: one that adds an exported func, one that
// only touches the docs, and one that reshapes the func added before it.
func commitScanRepo(t *testing.T) string {
	t.Helper()

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")
	runGit(t, root, "tag", "alpha/v0.1.0")

	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Greet greets.\nfunc Greet(name string) string { return name }\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: add Greet")

	writeTestFile(t, filepath.Join(alpha, "README.md"), "# alpha\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "alpha: document the package")

	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Greet greets.\nfunc Greet(name string, times int) string { return name }\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: greet a number of times")

	return alpha
}

func TestReadVerdictCountsTheAPIOfEachCommit(t *testing.T) {
	requireSplint(t)

	got, err := readVerdict(commitScanRepo(t), "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}
	if len(got.Commits) != 3 {
		t.Fatalf("readVerdict() Commits = %#v, want the three commits since the tag", got.Commits)
	}

	// The commits are newest first, so the range reads bottom up: Greet is
	// added, the docs are written, and the signature moves.
	want := []string{"+0/~1/-0", "", "+1/~0/-0"}
	for i, commit := range got.Commits {
		diff, ok := got.CommitAPI[commit.Hash]
		if !ok {
			t.Fatalf("readVerdict() scanned no API for %s %q", commit.Hash, commit.Subject)
		}
		if diff.Skipped != "" {
			t.Fatalf("readVerdict() skipped %s: %s", commit.Hash, diff.Skipped)
		}

		counts := symbolCounts(diff, true, false)
		if counts != want[i] {
			t.Errorf("readVerdict() %q = %q, want %q", commit.Subject, counts, want[i])
		}
	}
}

// twoModelsRepo builds a module holding two packages named model, one below
// the root and one below the front end, which is what the package column has
// to tell apart.
func twoModelsRepo(t *testing.T) string {
	t.Helper()

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")

	writeTestFile(t, filepath.Join(alpha, "model", "model.go"), "package model\n\n// Trace is a recorded trace.\ntype Trace struct{ ID string }\n")
	writeTestFile(t, filepath.Join(alpha, "frontend", "model", "model.go"), "package model\n\n// Page is a rendered page.\ntype Page struct{ Title string }\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "alpha: add the two models")

	return alpha
}
