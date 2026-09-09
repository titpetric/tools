package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecModulesRendersTable(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.25\n")
	chdir(t, root)

	var output bytes.Buffer
	failed := execModules(&output, map[string]string{"example.com/app": "."}, "true", false, false)

	if failed != 0 {
		t.Fatalf("execModules() failed = %d, want 0", failed)
	}
	want := "| Path | Module | Exec status |\n" +
		"| --- | --- | --- |\n" +
		"| . | example.com/app | Done. |\n"
	if got := output.String(); got != want {
		t.Fatalf("execModules() markdown =\n%s\nwant:\n%s", got, want)
	}
}

// TestExecModulesShowsOutput checks that the command's stdout and stderr end up
// in the status cell without asking for -v.
func TestExecModulesShowsOutput(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.25\n")
	chdir(t, root)

	var output bytes.Buffer
	execModules(&output, map[string]string{"example.com/app": "."}, "echo out; echo err >&2", false, false)

	got := output.String()
	for _, want := range []string{"out", "err"} {
		if !strings.Contains(got, want) {
			t.Fatalf("execModules() output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Done.") {
		t.Fatalf("execModules() printed Done. next to command output:\n%s", got)
	}
}

// TestExecModulesRunsInModuleDir checks that the command runs with the module
// directory as its working directory.
func TestExecModulesRunsInModuleDir(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sub", "go.mod"), "module example.com/sub\n\ngo 1.25\n")
	chdir(t, root)

	var output bytes.Buffer
	execModules(&output, map[string]string{"example.com/sub": "./sub"}, `basename "$PWD"`, false, false)

	if got := output.String(); !strings.Contains(got, "| sub |") {
		t.Fatalf("execModules() did not run in the module directory:\n%s", got)
	}
}

func TestExecModulesReportsFailures(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.25\n")
	chdir(t, root)

	var output bytes.Buffer
	failed := execModules(&output, map[string]string{"example.com/app": "."}, "echo broken >&2; exit 3", false, false)

	if failed != 1 {
		t.Fatalf("execModules() failed = %d, want 1", failed)
	}
	got := output.String()
	for _, want := range []string{"broken", "exit status 3"} {
		if !strings.Contains(got, want) {
			t.Fatalf("execModules() output missing %q:\n%s", want, got)
		}
	}
}
