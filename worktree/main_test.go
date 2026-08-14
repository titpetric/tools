package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

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
