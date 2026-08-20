package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/titpetric/tools/worktree/components"
)

func TestParseOptionsUpdateAllModules(t *testing.T) {
	originalArgs := os.Args
	originalFlags := flag.CommandLine
	t.Cleanup(func() {
		os.Args = originalArgs
		flag.CommandLine = originalFlags
	})

	os.Args = []string{"worktree", "-u", "./..."}
	flag.CommandLine = flag.NewFlagSet("worktree", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	opts := ParseOptions()
	if !opts.Update {
		t.Fatal("ParseOptions() did not enable dependency updates")
	}
	if opts.FilterPath != "" || opts.FilterArg != "" {
		t.Fatalf("ParseOptions() treated ./... as a filter: %#v", opts)
	}
}

// TestParseOptionsUpdateAllDeps checks that -U, the wider dependency update,
// implies -u.
func TestParseOptionsUpdateAllDeps(t *testing.T) {
	originalArgs := os.Args
	originalFlags := flag.CommandLine
	t.Cleanup(func() {
		os.Args = originalArgs
		flag.CommandLine = originalFlags
	})

	os.Args = []string{"worktree", "-U"}
	flag.CommandLine = flag.NewFlagSet("worktree", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	opts := ParseOptions()
	if !opts.UpdateAll || !opts.Update {
		t.Fatalf("ParseOptions(-U) = %#v, want both update flags set", opts)
	}
}

func TestParseOptionsVerbose(t *testing.T) {
	originalArgs := os.Args
	originalFlags := flag.CommandLine
	t.Cleanup(func() {
		os.Args = originalArgs
		flag.CommandLine = originalFlags
	})

	os.Args = []string{"worktree", "-v"}
	flag.CommandLine = flag.NewFlagSet("worktree", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	if opts := ParseOptions(); !opts.Verbose {
		t.Fatal("ParseOptions() did not enable verbose output")
	}
}

func TestParseOptionsDependencyMatrix(t *testing.T) {
	originalArgs := os.Args
	originalFlags := flag.CommandLine
	t.Cleanup(func() {
		os.Args = originalArgs
		flag.CommandLine = originalFlags
	})

	os.Args = []string{"worktree", "-t"}
	flag.CommandLine = flag.NewFlagSet("worktree", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	if opts := ParseOptions(); !opts.Matrix {
		t.Fatal("ParseOptions() did not enable dependency matrix output")
	}
}

func TestRenderDependencyMatrix(t *testing.T) {
	modules := []moduleInfo{
		{
			Name:     "example.com/service",
			Uses:     []string{"example.com/library"},
			GitState: &components.Git{Ahead: 1},
		},
		{Name: "example.com/library"},
		{
			Name:     "example.com/client",
			Uses:     []string{"example.com/library", "example.com/service"},
			GitState: &components.Git{DiffLines: []string{"go.mod +1/-1"}},
		},
		{Name: "example.com/tool", GitState: &components.Git{UntrackedFiles: []string{"PLAN.md"}}},
	}

	var output bytes.Buffer
	refs := versionRefs{
		"example.com/service": {"example.com/library": "v1.0.0"},
		"example.com/client":  {"example.com/library": "v2.0.0", "example.com/service": "v1.0.0"},
	}
	tags := latestTags{
		"example.com/library": "v2.0.0",
		"example.com/service": "v1.0.0",
	}
	renderDependencyMatrix(&output, modules, refs, tags, true)

	want := "" +
		"╭──────────────┬─────────┬─────────╮\n" +
		"│ Project      │ service │ library │\n" +
		"├──────────────┼─────────┼─────────┤\n" +
		"│ service (+1) │         │   ▲*    │\n" +
		"│ client *     │    ▲    │    ▲    │\n" +
		"│ tool *       │         │         │\n" +
		"╰──────────────┴─────────┴─────────╯\n" +
		"1 ahead, 2 with local changes, 1 deps out of date.\n"
	if got := ansi.Strip(output.String()); got != want {
		t.Fatalf("renderDependencyMatrix() =\n%s\nwant:\n%s", got, want)
	}
	if got := output.String(); !strings.Contains(got, components.ColorGreen+"▲") || !strings.Contains(got, components.ColorYellow+"▲*") {
		t.Fatalf("renderDependencyMatrix() did not color current and outdated dependencies: %q", got)
	}
	if got := output.String(); !strings.Contains(got, components.ColorSeparator+"(+1)") || !strings.Contains(got, components.ColorDarkOrange+"*") {
		t.Fatalf("renderDependencyMatrix() did not show project Git state: %q", got)
	}
	if got := output.String(); !strings.Contains(got, components.ColorHeader+"1 ahead, 2 with local changes, 1 deps out of date.") {
		t.Fatalf("renderDependencyMatrix() did not style summary: %q", got)
	}
}

func TestRenderDependencyMatrixMarkdown(t *testing.T) {
	modules := []moduleInfo{
		{Name: "example.com/app", Uses: []string{"example.com/lib"}},
		{Name: "example.com/lib"},
	}

	var output bytes.Buffer
	renderDependencyMatrix(&output, modules, nil, nil, false)

	want := "| Project | lib |\n" +
		"| --- | --- |\n" +
		"| app | ▲ |\n" +
		"0 ahead, 0 with local changes, 0 deps out of date.\n"
	if got := output.String(); got != want {
		t.Fatalf("renderDependencyMatrix() markdown =\n%s\nwant:\n%s", got, want)
	}
}

func TestPullReposRendersGitDetails(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	repo := filepath.Join(root, "service")
	runGit(t, root, "init", "--bare", remote)
	runGit(t, root, "clone", remote, repo)
	runGit(t, repo, "config", "user.name", "Test User")
	runGit(t, repo, "config", "user.email", "test@example.com")
	writeTestFile(t, filepath.Join(repo, "README.md"), "test\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "initial")
	runGit(t, repo, "push", "-u", "origin", "HEAD")
	updater := filepath.Join(root, "updater")
	runGit(t, root, "clone", remote, updater)
	runGit(t, updater, "config", "user.name", "Test User")
	runGit(t, updater, "config", "user.email", "test@example.com")
	writeTestFile(t, filepath.Join(updater, "CHANGELOG.md"), "update\n")
	runGit(t, updater, "add", "CHANGELOG.md")
	runGit(t, updater, "commit", "-m", "update")
	runGit(t, updater, "push")

	var output bytes.Buffer
	pullRepos(&output, []string{repo}, false)
	got := output.String()
	for _, want := range []string{
		"| Path | Remote | Branch | Pull status |",
		"origin " + remote + " (fetch)",
		"Pulled 1 commit.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("pullRepos() output missing %q:\n%s", want, got)
		}
	}

	output.Reset()
	pullRepos(&output, []string{repo}, false)
	if got := output.String(); !strings.Contains(got, "Already up to date.") {
		t.Fatalf("second pullRepos() did not report an up-to-date repository:\n%s", got)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatal(err)
		}
	})
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func TestRunCommandVerboseSuccess(t *testing.T) {
	var output bytes.Buffer
	cmd := exec.Command("go", "version")
	if err := runCommand(cmd, true, &output, &output); err != nil {
		t.Fatal(err)
	}

	want := "$ go version " + components.ColorGreen + "✓" + components.ColorReset + "\n"
	if !strings.HasSuffix(output.String(), want) {
		t.Fatalf("runCommand() output = %q, want suffix %q", output.String(), want)
	}
}

func TestRunCommandVerboseFailureHasNoCheckmark(t *testing.T) {
	var output bytes.Buffer
	cmd := exec.Command("go", "definitely-not-a-command")
	if err := runCommand(cmd, true, &output, &output); err == nil {
		t.Fatal("runCommand() unexpectedly succeeded")
	}

	if got, want := output.String(), "$ go definitely-not-a-command\n"; !strings.HasSuffix(got, want) {
		t.Fatalf("runCommand() output = %q, want suffix %q", got, want)
	}
}

func TestFindScanRootUsesNearestMarker(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.work"), "go 1.25\n")
	module := filepath.Join(root, "apps", "booking")
	writeTestFile(t, filepath.Join(module, "go.mod"), "module example.com/booking\n\ngo 1.25\n")
	start := filepath.Join(module, "cmd", "server")
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := findScanRoot(start)
	if err != nil {
		t.Fatal(err)
	}
	if got != module {
		t.Fatalf("findScanRoot() = %q, want %q", got, module)
	}
}

func TestFindProjectsIncludesGitRepositoriesAndGoModules(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/root\n\ngo 1.25\n")

	if err := os.MkdirAll(filepath.Join(root, "standalone", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "standalone", "nested", "go.mod"), "module example.com/nested\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, "module-only", "go.mod"), "module example.com/module-only\n\ngo 1.25\n")

	got, err := findProjects(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []projectDir{
		{Path: ".", GoModule: true, GitRepo: true},
		{Path: "." + string(filepath.Separator) + "module-only", GoModule: true},
		{Path: "." + string(filepath.Separator) + "standalone", GitRepo: true},
		{Path: "." + string(filepath.Separator) + filepath.Join("standalone", "nested"), GoModule: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findProjects() = %#v, want %#v", got, want)
	}
}

func TestFindProjectsSkipsIgnoredDirectories(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/root\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, ".gitignore"), "vendor/\n/tmp\n!apps/vendor\n")

	// Git repositories and Go modules inside an ignored folder are skipped.
	if err := os.MkdirAll(filepath.Join(root, "vendor", "example.com", "lib", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "vendor", "example.com", "lib", "go.mod"), "module example.com/lib\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, "tmp", "scratch", "go.mod"), "module example.com/scratch\n\ngo 1.25\n")

	// A negation in the same file re-includes the folder.
	writeTestFile(t, filepath.Join(root, "apps", "vendor", "go.mod"), "module example.com/apps/vendor\n\ngo 1.25\n")

	// An unanchored pattern matches at any depth, an anchored one does not.
	writeTestFile(t, filepath.Join(root, "libs", "vendor", "go.mod"), "module example.com/libs/vendor\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, "libs", "tmp", "go.mod"), "module example.com/libs/tmp\n\ngo 1.25\n")

	got, err := findProjects(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []projectDir{
		{Path: ".", GoModule: true},
		{Path: "." + string(filepath.Separator) + filepath.Join("apps", "vendor"), GoModule: true},
		{Path: "." + string(filepath.Separator) + filepath.Join("libs", "tmp"), GoModule: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findProjects() = %#v, want %#v", got, want)
	}
}

func TestFindProjectsIncludesGoWorkUseOutsideRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "workspace")
	external := filepath.Join(parent, "external")
	writeTestFile(t, filepath.Join(root, "go.work"), "go 1.25\n\nuse ../external\n")
	writeTestFile(t, filepath.Join(external, "go.mod"), "module example.com/external\n\ngo 1.25\n")

	got, err := findProjects(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []projectDir{{Path: "." + string(filepath.Separator) + filepath.Join("..", "external"), GoModule: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findProjects() = %#v, want %#v", got, want)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
