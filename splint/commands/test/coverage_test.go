package test

import (
	"regexp"
	"strings"
	"testing"
)

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
