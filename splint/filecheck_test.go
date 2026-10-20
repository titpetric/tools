package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	settings "github.com/titpetric/tools/splint/config"
)

// longFile is a Go source file of count declarations, which is well past any
// ceiling a test states and short enough to write in a temporary directory.
func longFile(count int) string {
	var out strings.Builder
	out.WriteString("package long\n")
	for i := range count {
		out.WriteString("\nfunc f")
		out.WriteString(string(rune('a' + i%26)))
		out.WriteString(string(rune('a' + (i/26)%26)))
		out.WriteString(string(rune('a' + (i/676)%26)))
		out.WriteString("() int {\n\treturn ")
		out.WriteString(string(rune('0' + i%10)))
		out.WriteString("\n}\n")
	}
	return out.String()
}

// longTree writes a tree holding one file over any stated ceiling, under the
// directory name each test writes its allow pattern against.
func longTree(t *testing.T, dir, name, config string) string {
	t.Helper()

	root := filepath.Join(t.TempDir(), "tree")
	target := filepath.Join(root, dir)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("building the tree: %v", err)
	}
	write := func(path, body string) {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	write(filepath.Join(root, "go.mod"), "module example.com/long\n\ngo 1.24\n")
	write(filepath.Join(target, name), longFile(200))
	if config != "" {
		write(settings.Path(root), config)
	}
	return root
}

// lint runs the lint command over root and answers what it reported.
func lint(t *testing.T, root string) string {
	t.Helper()

	var out, progress bytes.Buffer
	if _, err := run(context.Background(), []string{"-i", root, "./..."}, &out, &progress); err != nil {
		t.Fatalf("linting %s: %v", root, err)
	}
	return out.String()
}

// TestFilecheckCeilingIsTheDefaultWithNoConfig covers the tree that states
// nothing, which has to read exactly as it did before the section existed.
func TestFilecheckCeilingIsTheDefaultWithNoConfig(t *testing.T) {
	report := lint(t, longTree(t, "parser", "parser.go", ""))

	if !strings.Contains(report, "filecheck/long") {
		t.Fatalf("a 600-line file was not reported:\n%s", report)
	}
	if !strings.Contains(report, "over the 400 a file") {
		t.Errorf("the message names a ceiling other than the default:\n%s", report)
	}
}

// TestFilecheckCeilingFromTheConfig covers the wiring the consumer requested:
// a path named in .splint.yml runs under its own ceiling, and the message says
// which ceiling applied, and that ceiling differs from the default.
func TestFilecheckCeilingFromTheConfig(t *testing.T) {
	root := longTree(t, "parser", "parser.go", `filecheck:
  allow:
    - path: parser/*.go
      max-lines: 1500
`)
	if report := lint(t, root); strings.Contains(report, "filecheck/long") {
		t.Errorf("a file the config allowed was reported:\n%s", report)
	}

	// The same tree with a ceiling the file does not fit under fails the run,
	// and the number in the message is the one that applied.
	tight := longTree(t, "parser", "parser.go", `filecheck:
  allow:
    - path: parser/*.go
      max-lines: 450
`)
	report := lint(t, tight)
	if !strings.Contains(report, "over the 450 a file") {
		t.Errorf("the message does not name the ceiling that applied:\n%s", report)
	}
}

// TestFilecheckCeilingOffForAPath covers the zero on an entry, which takes the
// rule off the files it covers the way a zero takes a pollution rule off.
func TestFilecheckCeilingOffForAPath(t *testing.T) {
	root := longTree(t, "internal/generated", "api.go", `filecheck:
  allow:
    - path: internal/generated/*.go
      max-lines: 0
`)
	if report := lint(t, root); strings.Contains(report, "filecheck/long") {
		t.Errorf("a file the config took the rule off was reported:\n%s", report)
	}
}

// TestFilecheckCeilingLeavesTheRestOfTheTree covers the half of the contract
// that keeps the rule a rule: the default ceiling applies to a path no entry
// covers, so naming a parser does not stop the report on a new long file.
func TestFilecheckCeilingLeavesTheRestOfTheTree(t *testing.T) {
	root := longTree(t, "runner", "runner.go", `filecheck:
  allow:
    - path: parser/*.go
      max-lines: 1500
`)
	report := lint(t, root)
	if !strings.Contains(report, "filecheck/long") {
		t.Fatalf("a file no entry covers was not reported:\n%s", report)
	}
	if !strings.Contains(report, "over the 400 a file") {
		t.Errorf("the message names a ceiling other than the default:\n%s", report)
	}
}

// TestFilecheckConfigRefusedAtTheRoot covers a malformed pattern stopping the
// run at the file holding it. A pattern nothing can read would otherwise cover
// no file for the life of the file, and a tree would appear to state a rule.
func TestFilecheckConfigRefusedAtTheRoot(t *testing.T) {
	root := longTree(t, "parser", "parser.go", `filecheck:
  allow:
    - path: "parser/[.go"
      max-lines: 1500
`)
	var out, progress bytes.Buffer
	if _, err := run(context.Background(), []string{"-i", root, "./..."}, &out, &progress); err == nil {
		t.Fatalf("a malformed pattern was accepted, and the report was:\n%s", out.String())
	}
}
