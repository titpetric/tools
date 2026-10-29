package test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repo makes a git checkout with its first commit in it, so a test has a
// revision to compare a change against.
func repo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	gitRun(t, dir, "init", "-q", "-b", "main")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "user.name", "test")
	write(t, dir, "go.mod", "module example.com/t\n\ngo 1.21\n")
	commit(t, dir, "initial")
	return dir
}

// gitRun runs a git command in dir.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// commit records every change in dir.
func commit(t *testing.T, dir, message string) {
	t.Helper()
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-qm", message)
}

// write puts source at a path below dir, creating the directories above it.
func write(t *testing.T, dir, name, source string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
}

// list writes the tests a run would start.
func list(t *testing.T, opts Options) string {
	t.Helper()

	opts.List = true
	var out, said bytes.Buffer
	code, err := Run(context.Background(), opts, &out, &said)
	if err != nil {
		t.Fatalf("Run() error = %v\n%s", err, said.String())
	}
	if code != 0 {
		t.Fatalf("Run() exited %d, want 0", code)
	}
	return out.String()
}

// runStart runs the command and returns what it wrote and its exit code.
func runStart(t *testing.T, opts Options) (string, string, int) {
	t.Helper()

	var out, said bytes.Buffer
	code, err := Run(context.Background(), opts, &out, &said)
	if err != nil {
		t.Fatalf("Run() error = %v\n%s", err, said.String())
	}
	return out.String(), said.String(), code
}

// changedAdd is the repository the selection tests read: one function and its
// test, committed.
func changedAdd(t *testing.T) string {
	t.Helper()

	dir := repo(t)
	write(t, dir, "p.go", "package t\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n")
	write(t, dir, "p_test.go", "package t\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal(\"bad sum\")\n\t}\n}\n")
	commit(t, dir, "add")
	return dir
}

// TestWriteHelp covers the page a help request gets: the usage a caller reads
// and the flags splint itself takes.
func TestWriteHelp(t *testing.T) {
	var out bytes.Buffer
	if err := WriteHelp(&out); err != nil {
		t.Fatalf("WriteHelp() error = %v", err)
	}
	for _, want := range []string{"splint test", "--split", "--base", "--list"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the help page does not name %q:\n%s", want, out.String())
		}
	}
}

// TestParseOptionsForward covers the command line: splint's own flags are
// read and every other flag is one go test takes.
func TestParseOptionsForward(t *testing.T) {
	opts, help, err := ParseOptions([]string{"--split", "./...", "-count=1", "-timeout", "2m", "--base", "main"})
	if err != nil {
		t.Fatalf("ParseOptions() error = %v", err)
	}
	if help {
		t.Error("ParseOptions() asked for help")
	}
	if !opts.Split {
		t.Error("--split was not read")
	}
	if opts.Pattern != "./..." {
		t.Errorf("Pattern = %q, want ./...", opts.Pattern)
	}
	if opts.Base != "main" {
		t.Errorf("Base = %q, want main", opts.Base)
	}
	if got := strings.Join(opts.GoFlags, " "); got != "-count=1 -timeout 2m" {
		t.Errorf("GoFlags = %q, want %q", got, "-count=1 -timeout 2m")
	}
}

// TestParseOptionsSource covers the two spellings of the source path, and the
// one where the path is attached with an equals sign.
func TestParseOptionsSource(t *testing.T) {
	for _, args := range [][]string{{"-i", "tree"}, {"--i", "tree"}, {"-i=tree"}, {"--i=tree"}} {
		opts, _, err := ParseOptions(args)
		if err != nil {
			t.Fatalf("ParseOptions(%v) error = %v", args, err)
		}
		if opts.SourcePath != "tree" {
			t.Errorf("ParseOptions(%v) SourcePath = %q, want tree", args, opts.SourcePath)
		}
	}
}

// TestParseOptionsBooleanFlag covers a forwarded flag that takes no value: the
// pattern after it stays a pattern.
func TestParseOptionsBooleanFlag(t *testing.T) {
	opts, _, err := ParseOptions([]string{"-race", "./...", "-count", "1"})
	if err != nil {
		t.Fatalf("ParseOptions() error = %v", err)
	}
	if opts.Pattern != "./..." {
		t.Errorf("Pattern = %q, want ./...", opts.Pattern)
	}
	if got := strings.Join(opts.GoFlags, " "); got != "-race -count 1" {
		t.Errorf("GoFlags = %q, want %q", got, "-race -count 1")
	}
}
