package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// testRepo builds a git repository holding one go module per name, each in its
// own directory, and returns the repository root.
func testRepo(t *testing.T, modules ...string) string {
	t.Helper()

	root := t.TempDir()
	runGit(t, root, "init", "--quiet")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "test")

	for _, name := range modules {
		writeTestFile(t, filepath.Join(root, name, "go.mod"), "module example.com/"+name+"\n\ngo 1.24\n")
		writeTestFile(t, filepath.Join(root, name, name+".go"), "package "+name+"\n")
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "init")
	return root
}

func TestModuleTagsStripsTheNestedModulePrefix(t *testing.T) {
	root := testRepo(t, "alpha", "beta")
	runGit(t, root, "tag", "alpha/v0.1.0")
	runGit(t, root, "tag", "alpha/v0.2.0")
	runGit(t, root, "tag", "beta/v1.0.0")

	tags, prefix, err := moduleTags(filepath.Join(root, "alpha"))
	if err != nil {
		t.Fatalf("moduleTags() error: %v", err)
	}
	if prefix != "alpha/" {
		t.Errorf("moduleTags() prefix = %q, want %q", prefix, "alpha/")
	}
	if want := []string{"v0.1.0", "v0.2.0"}; !reflect.DeepEqual(tags, want) {
		t.Errorf("moduleTags() = %#v, want %#v", tags, want)
	}

	// The tags of a nested module are not semantic versions, so a module at
	// the root of the same repository does not pick them up.
	tags, prefix, err = moduleTags(root)
	if err != nil {
		t.Fatalf("moduleTags() error: %v", err)
	}
	if prefix != "" {
		t.Errorf("moduleTags() prefix = %q, want %q", prefix, "")
	}
	if _, found := LatestRelease(tags); found {
		t.Errorf("moduleTags() at the root found a release in %#v", tags)
	}
}

func TestReleaseStepsPrefixesANestedModuleTag(t *testing.T) {
	steps, err := releaseSteps([]string{"v0.1.0"}, releasePatch, "alpha/")
	if err != nil {
		t.Fatalf("releaseSteps() error: %v", err)
	}
	want := [][]string{{"git", "tag", "alpha/v0.1.1"}, {"git", "push", "--tags"}}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("releaseSteps() = %#v, want %#v", steps, want)
	}
}

func TestResolveRendersThePlanAndStopsOnADirtyModule(t *testing.T) {
	root := testRepo(t, "alpha", "beta")
	runGit(t, root, "tag", "alpha/v0.1.0")
	runGit(t, root, "tag", "beta/v0.2.0")

	writeTestFile(t, filepath.Join(root, "alpha", "alpha.go"), "package alpha\n\n// changed\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: change")

	// alpha is left holding a change resolve does not commit, so the run has
	// to stop there and never reach beta.
	writeTestFile(t, filepath.Join(root, "alpha", "stray.go"), "package alpha\n")

	chdir(t, root)
	var out bytes.Buffer
	err := resolve(&out, []moduleInfo{
		{Name: "example.com/alpha", Path: "./alpha"},
		{Name: "example.com/beta", Path: "./beta", Uses: []string{"example.com/alpha"}},
	}, versionRefs{"example.com/beta": {"example.com/alpha": "v0.1.0"}}, &Options{}, false)
	if err != nil {
		t.Fatalf("resolve() error: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"| Path | Module | Release | Resolution |",
		"| ./alpha | example.com/alpha | v0.1.0 (+1) → v0.1.1 |",
		"go get -u ./...",
		"go mod tidy",
		// The module has no go.sum, so only the file that exists is named,
		// and the message reads as the one argument it is.
		"git add -- go.mod",
		"git commit -m 'alpha: update go.mod' -- go.mod",
		"?? stray.go",
		"Stopped: working tree is dirty.",
		"| ./beta | example.com/beta |",
		"Not reached, the run stopped at example.com/alpha.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("resolve() output missing %q:\n%s", want, got)
		}
	}

	// Nothing after the stop is proposed, since it would pin a tag that
	// never gets created.
	if strings.Contains(got, "git tag") {
		t.Errorf("resolve() proposed a release for a module it stopped at:\n%s", got)
	}
}

func TestResolveLeavesOutModulesWithNothingToDo(t *testing.T) {
	root := testRepo(t, "alpha", "beta")
	runGit(t, root, "tag", "alpha/v0.1.0")

	chdir(t, root)
	modules := []moduleInfo{
		{Name: "example.com/alpha", Path: "./alpha"},
		{Name: "example.com/beta", Path: "./beta"},
	}

	// beta has no release tag, so there is nothing to update it towards and
	// nothing downstream could pin to it.
	var out bytes.Buffer
	if err := resolve(&out, modules, versionRefs{}, &Options{}, false); err != nil {
		t.Fatalf("resolve() error: %v", err)
	}
	if got := out.String(); strings.Contains(got, "example.com/beta") {
		t.Errorf("resolve() listed a module with nothing to do:\n%s", got)
	}

	// --all is what asks for them, as it does everywhere else in the tool.
	out.Reset()
	if err := resolve(&out, modules, versionRefs{}, &Options{All: true}, false); err != nil {
		t.Fatalf("resolve() error: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "no release tag") {
		t.Errorf("resolve(--all) left out a module with nothing to do:\n%s", got)
	}
}

func TestResolveOffersTheUpdateToAReleasedModule(t *testing.T) {
	root := testRepo(t, "alpha")
	runGit(t, root, "tag", "alpha/v0.1.0")

	chdir(t, root)
	var out bytes.Buffer
	if err := resolve(&out, []moduleInfo{{Name: "example.com/alpha", Path: "./alpha"}}, versionRefs{}, &Options{}, false); err != nil {
		t.Fatalf("resolve() error: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"go get -u ./...",
		"go mod tidy",
		"released only if go.mod, go.sum change",
		"git tag alpha/v0.1.1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("resolve() output missing %q:\n%s", want, got)
		}
	}
}

func TestResolveTidiesAfterEveryGet(t *testing.T) {
	root := testRepo(t, "alpha", "beta")
	runGit(t, root, "tag", "alpha/v0.1.0")
	runGit(t, root, "tag", "beta/v0.2.0")
	writeTestFile(t, filepath.Join(root, "beta", "beta.go"), "package beta\n\n// changed\n")
	runGit(t, root, "commit", "--quiet", "-am", "beta: change")

	chdir(t, root)
	var out bytes.Buffer
	if err := resolve(&out, []moduleInfo{
		{Name: "example.com/alpha", Path: "./alpha"},
		{Name: "example.com/beta", Path: "./beta", Uses: []string{"example.com/alpha"}},
	}, versionRefs{"example.com/beta": {"example.com/alpha": "v0.0.1"}}, &Options{}, false); err != nil {
		t.Fatalf("resolve() error: %v", err)
	}

	// Each go get is tidied straight after it, so the requirement it replaced
	// is out of go.mod before the next one is asked for.
	want := "go get -u ./...<br>go mod tidy<br>go get example.com/alpha@v0.1.1<br>go mod tidy<br>"
	if got := out.String(); !strings.Contains(got, want) {
		t.Errorf("resolve() output missing %q:\n%s", want, got)
	}
}

func TestShellJoin(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"go", "mod", "tidy"}, want: "go mod tidy"},
		// Nothing a shell would expand, so nothing to quote.
		{args: []string{"go", "get", "-u", "./..."}, want: "go get -u ./..."},
		{args: []string{"go", "get", "example.com/x@v1.0.0"}, want: "go get example.com/x@v1.0.0"},
		{args: []string{"git", "commit", "-m", "alpha: update go.mod"}, want: "git commit -m 'alpha: update go.mod'"},
		{args: []string{"echo", "it's"}, want: `echo 'it'\''s'`},
		{args: []string{"echo", ""}, want: "echo ''"},
	}

	for _, test := range tests {
		if got := shellJoin(test.args); got != test.want {
			t.Errorf("shellJoin(%#v) = %q, want %q", test.args, got, test.want)
		}
	}
}

func TestModPathsNamesTheFilesThatExist(t *testing.T) {
	dir := t.TempDir()
	if got := modPaths(dir); got != nil {
		t.Fatalf("modPaths() on an empty directory = %#v, want nil", got)
	}

	writeTestFile(t, filepath.Join(dir, "go.mod"), "module example.com/x\n")
	if got := modPaths(dir); !reflect.DeepEqual(got, []string{"go.mod"}) {
		t.Fatalf("modPaths() = %#v, want [go.mod]", got)
	}

	// A go.sum the update creates is untracked, which is why it is staged
	// rather than left to a pathspec that would not match it.
	writeTestFile(t, filepath.Join(dir, "go.sum"), "\n")
	if got := modPaths(dir); !reflect.DeepEqual(got, []string{"go.mod", "go.sum"}) {
		t.Fatalf("modPaths() = %#v, want [go.mod go.sum]", got)
	}
}

func TestResolveCommitsAnUntrackedGoSumRatherThanStopping(t *testing.T) {
	root := testRepo(t, "alpha")
	runGit(t, root, "tag", "alpha/v0.1.0")
	writeTestFile(t, filepath.Join(root, "alpha", "alpha.go"), "package alpha\n\n// changed\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: change")

	// go mod tidy leaves a go.sum behind that git has never seen.
	writeTestFile(t, filepath.Join(root, "alpha", "go.sum"), "\n")

	chdir(t, root)
	var out bytes.Buffer
	if err := resolve(&out, []moduleInfo{{Name: "example.com/alpha", Path: "./alpha"}}, versionRefs{}, &Options{}, false); err != nil {
		t.Fatalf("resolve() error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "git add -- go.mod go.sum") {
		t.Errorf("resolve() did not stage the new go.sum:\n%s", got)
	}
	if strings.Contains(got, "working tree is dirty") {
		t.Errorf("resolve() treated the go.sum it commits itself as a dirty tree:\n%s", got)
	}
	if !strings.Contains(got, "git tag alpha/v0.1.1") {
		t.Errorf("resolve() did not propose a release:\n%s", got)
	}
}

func TestParseOptionsResolve(t *testing.T) {
	tests := []struct {
		args   []string
		apply  bool
		filter string
	}{
		{args: []string{"worktree", "resolve"}},
		{args: []string{"worktree", "resolve", "--apply"}, apply: true},
		{args: []string{"worktree", "resolve", "./..."}},
		{args: []string{"worktree", "resolve", "platform"}, filter: "platform"},
		{args: []string{"worktree", "resolve", "--apply", "platform"}, apply: true, filter: "platform"},
		{args: []string{"worktree", "resolve", "platform", "--apply"}, apply: true, filter: "platform"},
	}

	for _, test := range tests {
		func() {
			args, commandLine := os.Args, flag.CommandLine
			t.Cleanup(func() { os.Args, flag.CommandLine = args, commandLine })

			os.Args = test.args
			flag.CommandLine = flag.NewFlagSet("worktree", flag.ContinueOnError)
			flag.CommandLine.SetOutput(io.Discard)

			opts := ParseOptions()
			if !opts.Resolve {
				t.Fatalf("%v: Resolve = false, want true", test.args)
			}
			if opts.Apply != test.apply {
				t.Errorf("%v: Apply = %v, want %v", test.args, opts.Apply, test.apply)
			}
			if opts.FilterArg != test.filter {
				t.Errorf("%v: FilterArg = %q, want %q", test.args, opts.FilterArg, test.filter)
			}
		}()
	}
}

func TestResolveAlignsTheGoVersionAcrossModules(t *testing.T) {
	root := testRepo(t, "alpha", "beta")
	runGit(t, root, "tag", "alpha/v0.1.0")
	runGit(t, root, "tag", "beta/v0.2.0")

	// beta declares the newer version, so alpha is brought up to it.
	writeTestFile(t, filepath.Join(root, "beta", "go.mod"), "module example.com/beta\n\ngo 1.27\n")
	runGit(t, root, "commit", "--quiet", "-am", "beta: move to go 1.27")

	chdir(t, root)
	modules := []moduleInfo{
		{Name: "example.com/alpha", Path: "./alpha", GoVersion: "1.24"},
		{Name: "example.com/beta", Path: "./beta", GoVersion: "1.27"},
	}

	plans, _ := planResolve(modules, versionRefs{})
	alpha, beta := plans[0], plans[1]

	if alpha.GoFrom != "1.24" || alpha.GoTo != "1.27" {
		t.Errorf("alpha = {GoFrom: %q, GoTo: %q}, want {1.24, 1.27}", alpha.GoFrom, alpha.GoTo)
	}
	// Moving to another release series stops the module building for anyone
	// on the older toolchain, so it costs a minor.
	if alpha.Release != releaseMinor || alpha.Next != "v0.2.0" {
		t.Errorf("alpha = {Release: %q, Next: %q}, want {minor, v0.2.0}", alpha.Release, alpha.Next)
	}
	if alpha.Conditional {
		t.Error("a module with a go directive to raise was left conditional")
	}

	// beta already declares it, so it has nothing to raise, but the version
	// it raised by hand since its tag still costs it a minor.
	if beta.GoTo != "" {
		t.Errorf("beta.GoTo = %q, want it empty", beta.GoTo)
	}
	if beta.GoSince != "1.24" || beta.Release != releaseMinor {
		t.Errorf("beta = {GoSince: %q, Release: %q}, want {1.24, minor}", beta.GoSince, beta.Release)
	}

	var out bytes.Buffer
	if err := resolve(&out, modules, versionRefs{}, &Options{}, false); err != nil {
		t.Fatalf("resolve() error: %v", err)
	}
	got := out.String()
	for _, want := range []string{"go mod edit -go=1.27", "go 1.24 → 1.27", "go 1.24 → 1.27 since v0.2.0"} {
		if !strings.Contains(got, want) {
			t.Errorf("resolve() output missing %q:\n%s", want, got)
		}
	}
}
