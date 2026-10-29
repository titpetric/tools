package test

import (
	"strings"
	"testing"
)

// TestListSelectsReceiverClass covers a method change: every Test<Type>* runs,
// which is the class of tests bound to the receiver.
func TestListSelectsReceiverClass(t *testing.T) {
	dir := repo(t)
	write(t, dir, "w.go", "package t\n\ntype Worker struct{}\n\nfunc (w Worker) Run() {}\n\nfunc (w Worker) Stop() {}\n")
	write(t, dir, "w_test.go", "package t\n\nimport \"testing\"\n\nfunc TestWorkerRun(t *testing.T) { Worker{}.Run() }\n\nfunc TestWorkerStop(t *testing.T) { Worker{}.Stop() }\n\nfunc TestUnrelated(t *testing.T) {}\n")
	commit(t, dir, "worker")
	write(t, dir, "w.go", "package t\n\ntype Worker struct{}\n\nfunc (w Worker) Run() { _ = w }\n\nfunc (w Worker) Stop() {}\n")

	got := list(t, Options{SourcePath: dir, Pattern: "./..."})
	for _, want := range []string{"TestWorkerRun", "TestWorkerStop"} {
		if !strings.Contains(got, want+"\n") {
			t.Errorf("a method change did not select %s:\n%s", want, got)
		}
	}
	if strings.Contains(got, "TestUnrelated") {
		t.Errorf("a method change selected a test of another class:\n%s", got)
	}
}

// TestListSelectsChangedTest covers the test that changed itself: it runs and
// its package's other tests stay out.
func TestListSelectsChangedTest(t *testing.T) {
	dir := repo(t)
	write(t, dir, "p.go", "package t\n\nfunc Add(a, b int) int { return a + b }\n")
	write(t, dir, "p_test.go", "package t\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) { Add(1, 2) }\n\nfunc TestOther(t *testing.T) {}\n")
	commit(t, dir, "initial tests")
	write(t, dir, "p_test.go", "package t\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) { Add(1, 2); t.Log(\"sum\") }\n\nfunc TestOther(t *testing.T) {}\n")

	got := list(t, Options{SourcePath: dir, Pattern: "./..."})
	if strings.TrimSpace(got) != "TestAdd" {
		t.Fatalf("a changed test selected %q, want TestAdd", got)
	}
}

// TestListSelectsChangedBenchmark covers the benchmark that changed: it is a
// benchmark selection, and the run gives it -benchtime 10s.
func TestListSelectsChangedBenchmark(t *testing.T) {
	dir := repo(t)
	write(t, dir, "p.go", "package t\n\nfunc Add(a, b int) int { return a + b }\n")
	write(t, dir, "p_test.go", "package t\n\nimport \"testing\"\n\nfunc BenchmarkAdd(b *testing.B) { for i := 0; i < b.N; i++ { Add(1, 2) } }\n")
	commit(t, dir, "benchmark")
	write(t, dir, "p_test.go", "package t\n\nimport \"testing\"\n\nfunc BenchmarkAdd(b *testing.B) { for i := 0; i < b.N; i++ { Add(2, 1) } }\n")

	got := list(t, Options{SourcePath: dir, Pattern: "./..."})
	if strings.TrimSpace(got) != "BenchmarkAdd" {
		t.Fatalf("a changed benchmark selected %q, want BenchmarkAdd", got)
	}
}

// TestListSelectsCrossPackageTest covers a test in another package that calls
// the changed exported function through the import of its package.
func TestListSelectsCrossPackageTest(t *testing.T) {
	dir := repo(t)
	write(t, dir, "p/p.go", "package p\n\nfunc Hello() string { return \"hi\" }\n")
	write(t, dir, "q/q_test.go", "package q\n\nimport (\n\t\"testing\"\n\n\t\"example.com/t/p\"\n)\n\nfunc TestHello(t *testing.T) {\n\tif p.Hello() != \"hi\" {\n\t\tt.Fatal(\"greeting\")\n\t}\n}\n")
	commit(t, dir, "packages")
	write(t, dir, "p/p.go", "package p\n\nfunc Hello() string { return \"hi\" + \"\" }\n")

	got := list(t, Options{SourcePath: dir, Pattern: "./..."})
	if !strings.Contains(got, "TestHello\n") {
		t.Fatalf("a cross-package test was not selected:\n%s", got)
	}
}
