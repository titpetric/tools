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
		{Name: "example.com/service", Uses: []string{"example.com/library"}},
		{Name: "example.com/library"},
		{Name: "example.com/client", Uses: []string{"example.com/library", "example.com/service"}},
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
	renderDependencyMatrix(&output, modules, refs, tags)

	want := "" +
		"| Project | service | library |\n" +
		"| ------- | :-----: | :-----: |\n" +
		"| service |         |    " + components.ColorYellow + "▲" + components.ColorReset + "    |\n" +
		"| client  |    " + components.ColorGreen + "▲" + components.ColorReset + "    |    " + components.ColorGreen + "▲" + components.ColorReset + "    |\n"
	if got := output.String(); got != want {
		t.Fatalf("renderDependencyMatrix() =\n%s\nwant:\n%s", got, want)
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
