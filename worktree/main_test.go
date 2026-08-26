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
	"github.com/titpetric/tools/worktree/config"
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

// TestParseOptionsVerboseLongFlag checks that the spelled out --verbose is
// accepted, since that is how the flag reads in the documentation.
func TestParseOptionsVerboseLongFlag(t *testing.T) {
	originalArgs := os.Args
	originalFlags := flag.CommandLine
	t.Cleanup(func() {
		os.Args = originalArgs
		flag.CommandLine = originalFlags
	})

	os.Args = []string{"worktree", "--verbose"}
	flag.CommandLine = flag.NewFlagSet("worktree", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	if opts := ParseOptions(); !opts.Verbose {
		t.Fatal("ParseOptions(--verbose) did not enable verbose output")
	}
}

// TestParseOptionsConfigure checks the config subcommand opens the setup
// screen and is not mistaken for a path filter.
func TestParseOptionsConfigure(t *testing.T) {
	originalArgs := os.Args
	originalFlags := flag.CommandLine
	t.Cleanup(func() {
		os.Args = originalArgs
		flag.CommandLine = originalFlags
	})

	os.Args = []string{"worktree", commandConfig}
	flag.CommandLine = flag.NewFlagSet("worktree", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	opts := ParseOptions()
	if !opts.Configure {
		t.Fatal("ParseOptions() did not select the setup screen")
	}
	if opts.FilterArg != "" || opts.FilterPath != "" {
		t.Fatalf("ParseOptions() treated %q as a filter: %#v", commandConfig, opts)
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
		"│ service (+1) │         │ ▲*      │\n" +
		"│ client *     │ ▲       │ ▲       │\n" +
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

func TestRenderDependencyMatrixMinWidth(t *testing.T) {
	modules := []moduleInfo{
		{Name: "example.com/app", Uses: []string{"example.com/db"}},
		{Name: "example.com/db"},
	}

	var output bytes.Buffer
	renderDependencyMatrix(&output, modules, nil, nil, true)

	want := "" +
		"╭─────────┬─────╮\n" +
		"│ Project │ db  │\n" +
		"├─────────┼─────┤\n" +
		"│ app     │ ▲   │\n" +
		"╰─────────┴─────╯\n" +
		"0 ahead, 0 with local changes, 0 deps out of date.\n"
	if got := ansi.Strip(output.String()); got != want {
		t.Fatalf("renderDependencyMatrix() =\n%s\nwant:\n%s", got, want)
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

	got, err := findScanRoot(start, config.Default().Scan.RootMarkers)
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

	got, err := findProjects(root, config.Default().Scan)
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

	got, err := findProjects(root, config.Default().Scan)
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

// TestFindProjectsWithoutGitignore checks the setting that turns the
// .gitignore rules off. A workspace that consolidates git checkouts below it
// gitignores those folders so they stay out of its own index, and with the
// rules on their checkouts are never listed.
func TestFindProjectsWithoutGitignore(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".gitignore"), "checkouts/\n")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "checkouts", "lib", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "checkouts", "lib", "go.mod"), "module example.com/lib\n\ngo 1.25\n")

	scan := config.Default().Scan
	nested := projectDir{
		Path:     "." + string(filepath.Separator) + filepath.Join("checkouts", "lib"),
		GoModule: true,
		GitRepo:  true,
	}

	// With the rules on, the checkout is hidden.
	got, err := findProjects(root, scan)
	if err != nil {
		t.Fatal(err)
	}
	if want := []projectDir{{Path: ".", GitRepo: true}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findProjects() = %#v, want %#v", got, want)
	}

	// With them off, it is listed.
	scan.EnableGitignore = false
	got, err = findProjects(root, scan)
	if err != nil {
		t.Fatal(err)
	}
	want := []projectDir{{Path: ".", GitRepo: true}, nested}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findProjects() = %#v, want %#v", got, want)
	}
}

// TestFindProjectsSkipsIgnorePaths checks the configured ignore paths exclude
// a directory no .gitignore mentions, which is how a listing is kept clean
// once the .gitignore rules are off.
func TestFindProjectsSkipsIgnorePaths(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/root\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, "node_modules", "dep", "go.mod"), "module example.com/dep\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, "apps", "go.mod"), "module example.com/apps\n\ngo 1.25\n")

	scan := config.Default().Scan
	scan.EnableGitignore = false
	scan.IgnorePaths = []string{"node_modules"}

	got, err := findProjects(root, scan)
	if err != nil {
		t.Fatal(err)
	}
	want := []projectDir{
		{Path: ".", GoModule: true},
		{Path: "." + string(filepath.Separator) + "apps", GoModule: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findProjects() = %#v, want %#v", got, want)
	}
}

// TestFindProjectsWithoutGitRepos checks the setting that drops git
// repositories holding no go module, leaving a go only listing.
func TestFindProjectsWithoutGitRepos(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/root\n\ngo 1.25\n")
	if err := os.MkdirAll(filepath.Join(root, "docs", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	scan := config.Default().Scan
	scan.EnableGitRepos = false

	got, err := findProjects(root, scan)
	if err != nil {
		t.Fatal(err)
	}
	if want := []projectDir{{Path: ".", GoModule: true}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findProjects() = %#v, want %#v", got, want)
	}
}

// TestFindScanRootUsesConfiguredMarkers checks the root markers come from the
// configuration, and that a walk with none starts where it was asked to.
func TestFindScanRootUsesConfiguredMarkers(t *testing.T) {
	root := t.TempDir()
	start := filepath.Join(root, "nested", "deep")
	writeTestFile(t, filepath.Join(root, "go.work"), "go 1.25\n")
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := findScanRoot(start, []string{"go.work"})
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("findScanRoot() = %q, want %q", got, root)
	}

	// A marker the workspace does not hold walks up to the filesystem root
	// and falls back to the directory it started in.
	if got, err = findScanRoot(start, []string{"Cargo.toml"}); err != nil || got != start {
		t.Fatalf("findScanRoot() = %q, %v, want %q, nil", got, err, start)
	}
	if got, err = findScanRoot(start, nil); err != nil || got != start {
		t.Fatalf("findScanRoot() with no markers = %q, %v, want %q, nil", got, err, start)
	}
}

func TestFindProjectsIncludesGoWorkUseOutsideRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "workspace")
	external := filepath.Join(parent, "external")
	writeTestFile(t, filepath.Join(root, "go.work"), "go 1.25\n\nuse ../external\n")
	writeTestFile(t, filepath.Join(external, "go.mod"), "module example.com/external\n\ngo 1.25\n")

	got, err := findProjects(root, config.Default().Scan)
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
