package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
