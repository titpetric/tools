package test

import (
	"regexp"
	"strings"
	"testing"
)

// TestStatusLines covers the verdicts go test -v writes: a subtest is part of
// its parent, and a passing subtest does not take back a failing parent.
func TestStatusLines(t *testing.T) {
	out := []byte(`--- PASS: TestPass (0.00s)
--- FAIL: TestFail (0.00s)
--- PASS: TestFail/sub (0.00s)
--- SKIP: TestSkip (0.00s)
`)
	got := statusLines(out)
	for name, want := range map[string]bool{"TestPass": true, "TestFail": false, "TestSkip": true} {
		if got[name] != want {
			t.Errorf("%s = %v, want %v", name, got[name], want)
		}
	}
	if _, found := got["TestFail/sub"]; found {
		t.Error("a subtest got a verdict of its own")
	}
}

// TestRunReportsVerdicts covers the lines a run writes: one PASS or FAIL for
// each test it started, and the exit code that goes with them.
func TestRunReportsVerdicts(t *testing.T) {
	dir := changedAdd(t)
	write(t, dir, "p.go", "package t\n\nfunc Add(a, b int) int {\n\treturn b + a\n}\n")

	got, _, code := runStart(t, Options{SourcePath: dir, Pattern: "./..."})
	if code != 0 {
		t.Fatalf("Run() exited %d, want 0", code)
	}
	if !strings.Contains(got, "PASS TestAdd\n") {
		t.Fatalf("Run() did not report PASS TestAdd:\n%s", got)
	}

	write(t, dir, "p.go", "package t\n\nfunc Add(a, b int) int {\n\treturn a + b + 1\n}\n")
	got, said, code := runStart(t, Options{SourcePath: dir, Pattern: "./..."})
	if code != 1 {
		t.Fatalf("a failing test exited %d, want 1", code)
	}
	if !strings.Contains(got, "FAIL TestAdd\n") {
		t.Fatalf("Run() did not report FAIL TestAdd:\n%s", got)
	}
	if !strings.Contains(said, "bad sum") {
		t.Errorf("the failing run did not carry what go test said:\n%s", said)
	}
}

// TestRunFindsModule covers a tree holding a module per project and no go.mod
// at its root, the layout a repository of tools uses. go test resolves a module
// from the directory it runs in, so the run has to start in the package's.
func TestRunFindsModule(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q", "-b", "main")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "user.name", "test")

	write(t, dir, "tool/go.mod", "module example.com/tool\n\ngo 1.21\n")
	write(t, dir, "tool/p.go", "package tool\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n")
	write(t, dir, "tool/p_test.go", "package tool\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) { Add(1, 2) }\n")
	commit(t, dir, "initial")

	write(t, dir, "tool/p.go", "package tool\n\nfunc Add(a, b int) int {\n\treturn b + a\n}\n")

	out, said, code := runStart(t, Options{SourcePath: dir, Pattern: "./..."})
	if code != 0 {
		t.Fatalf("Run() exited %d, want 0\n%s\n%s", code, out, said)
	}
	if !strings.Contains(out, "PASS TestAdd\n") {
		t.Fatalf("Run() did not report PASS TestAdd:\n%s\n%s", out, said)
	}
}

// TestSplitRunsOneCommandPerTest covers --split: every test is an invocation of
// its own.
func TestSplitRunsOneCommandPerTest(t *testing.T) {
	dir := repo(t)
	write(t, dir, "p.go", "package t\n\nfunc Add(a, b int) int { return a + b }\n\nfunc Mul(a, b int) int { return a * b }\n")
	write(t, dir, "p_test.go", "package t\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) { Add(1, 2) }\n\nfunc TestMul(t *testing.T) { Mul(1, 2) }\n")
	commit(t, dir, "functions")
	write(t, dir, "p.go", "package t\n\nfunc Add(a, b int) int { return b + a }\n\nfunc Mul(a, b int) int { return b * a }\n")

	out, said, code := runStart(t, Options{SourcePath: dir, Pattern: "./...", Split: true, Verbose: true})
	if code != 0 {
		t.Fatalf("Run() exited %d, want 0\n%s\n%s", code, out, said)
	}
	if commands := strings.Count(said, "splint test: go test"); commands != 2 {
		t.Errorf("--split ran %d commands, want 2:\n%s", commands, said)
	}
}

// TestScanReportsCoverage covers the coverage merged into the scan: one row per
// changed symbol, and the share of the tree's tests that ran.
func TestScanReportsCoverage(t *testing.T) {
	dir := repo(t)
	write(t, dir, "p.go", "package t\n\nfunc Add(a, b int) int {\n\treturn helper(a, b)\n}\n\nfunc helper(a, b int) int {\n\treturn a + b\n}\n")
	write(t, dir, "p_test.go", "package t\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal(\"bad sum\")\n\t}\n}\n\nfunc TestOther(t *testing.T) {}\n")
	commit(t, dir, "functions")
	write(t, dir, "p.go", "package t\n\nfunc Add(a, b int) int {\n\treturn helper(a, b)\n}\n\nfunc helper(a, b int) int {\n\treturn b + a\n}\n")

	out, said, code := runStart(t, Options{SourcePath: dir, Pattern: "./..."})
	if code != 0 {
		t.Fatalf("Run() exited %d, want 0\n%s\n%s", code, out, said)
	}

	row := regexp.MustCompile(`(?m)^helper\s+indirect\s+\d+/\d+\s+[\d.]+%`)
	if !row.MatchString(out) {
		t.Errorf("the scan did not mark helper indirect with coverage:\n%s", out)
	}
	if !strings.Contains(out, "Tests: 1/2 ran (50.0%)") {
		t.Errorf("the scan did not report the tests ran:\n%s", out)
	}
}

// TestScanMarksInternalMethodIndirect covers an internal method: its coverage
// is printed as indirect, through the function on the type that calls it.
func TestScanMarksInternalMethodIndirect(t *testing.T) {
	dir := repo(t)
	write(t, dir, "w.go", "package t\n\ntype Worker struct{}\n\nfunc (w Worker) Run() { w.step() }\n\nfunc (w Worker) step() {}\n")
	write(t, dir, "w_test.go", "package t\n\nimport \"testing\"\n\nfunc TestRun(t *testing.T) { Worker{}.Run() }\n")
	commit(t, dir, "worker")
	write(t, dir, "w.go", "package t\n\ntype Worker struct{}\n\nfunc (w Worker) Run() { w.step() }\n\nfunc (w Worker) step() { _ = w }\n")

	out, said, code := runStart(t, Options{SourcePath: dir, Pattern: "./..."})
	if code != 0 {
		t.Fatalf("Run() exited %d, want 0\n%s\n%s", code, out, said)
	}

	row := regexp.MustCompile(`(?m)^Worker\.step\s+indirect \(via Worker\.Run\)\s+\d+/\d+`)
	if !row.MatchString(out) {
		t.Errorf("the scan did not mark the internal method indirect through its invoker:\n%s", out)
	}
}
