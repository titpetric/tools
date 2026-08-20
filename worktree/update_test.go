package main

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/titpetric/tools/worktree/components"
)

func TestDiffRequires(t *testing.T) {
	before := []requireInfo{
		{path: "example.com/kept", version: "v1.0.0"},
		{path: "example.com/lib", version: "v1.0.0"},
		{path: "example.com/removed", version: "v0.1.0"},
	}
	after := []requireInfo{
		{path: "example.com/added", version: "v0.2.0"},
		{path: "example.com/kept", version: "v1.0.0"},
		{path: "example.com/lib", version: "v1.2.0"},
	}

	var got []string
	for _, change := range diffRequires(before, after) {
		got = append(got, change.String())
	}
	want := []string{
		"+ example.com/added v0.2.0",
		"example.com/lib v1.0.0 → v1.2.0",
		"- example.com/removed v0.1.0",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffRequires() = %#v, want %#v", got, want)
	}
}

func TestDepChangeColor(t *testing.T) {
	changes := map[string]depChange{
		components.ColorAmber:     {path: "example.com/lib", from: "v1.0.0", to: "v1.2.0"},
		components.ColorGreen:     {path: "example.com/added", to: "v0.2.0"},
		components.ColorSeparator: {path: "example.com/removed", from: "v0.1.0"},
	}
	for want, change := range changes {
		if got := change.Color(); got != want {
			t.Fatalf("depChange{%s}.Color() = %q, want %q", change, got, want)
		}
	}
}

func TestUpdateDepsRendersTable(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}\n")
	chdir(t, root)

	var output bytes.Buffer
	updateDeps(&output, map[string]string{"example.com/app": "."}, nil, &Options{Update: true}, false)

	want := "| Path | Module | Update status |\n" +
		"| --- | --- | --- |\n" +
		"| . | example.com/app | Already up to date. |\n"
	if got := output.String(); got != want {
		t.Fatalf("updateDeps() markdown =\n%s\nwant:\n%s", got, want)
	}
}

// TestUpdateDepsStripsModuleHost checks that the module column of the update
// table drops the "github.com/" prefix of an import path.
func TestUpdateDepsStripsModuleHost(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module github.com/titpetric/tools\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}\n")
	chdir(t, root)

	var output bytes.Buffer
	updateDeps(&output, map[string]string{"github.com/titpetric/tools": "."}, nil, &Options{Update: true}, false)

	got := output.String()
	if strings.Contains(got, "github.com/") {
		t.Fatalf("updateDeps() kept the github.com/ prefix:\n%s", got)
	}
	if !strings.Contains(got, "| titpetric/tools |") {
		t.Fatalf("updateDeps() output missing the module path:\n%s", got)
	}
}

func TestUpdateDepsReportsFailures(t *testing.T) {
	chdir(t, t.TempDir())

	var output bytes.Buffer
	updateDeps(&output, map[string]string{"example.com/missing": "./missing"}, nil, &Options{Update: true}, false)

	got := output.String()
	if strings.Contains(got, "Already up to date.") {
		t.Fatalf("updateDeps() reported success for a missing module:\n%s", got)
	}
	if want := "failed to read go.mod"; !strings.Contains(got, want) {
		t.Fatalf("updateDeps() output missing %q:\n%s", want, got)
	}
}

// TestUpdateDepsUpdatesOnlyStaleRequires checks that -u moves the workspace
// requirements that are behind their latest tag and leaves everything else
// alone. The module also requires a dependency outside the workspace, which
// has no known tag and is therefore not touched.
func TestUpdateDepsUpdatesOnlyStaleRequires(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.25\n\nrequire (\n\texample.com/current v1.1.0\n\texample.com/lib v1.0.0\n\texample.com/third-party v0.3.0\n)\n")
	t.Setenv("GOPROXY", "off")
	chdir(t, root)

	tags := latestTags{
		"example.com/current": "v1.1.0",
		"example.com/lib":     "v1.2.0",
	}

	var output bytes.Buffer
	updateDeps(&output, map[string]string{"example.com/app": "."}, tags, &Options{Update: true, Verbose: true}, false)

	got := output.String()
	if want := "$ go get example.com/lib@v1.2.0"; !strings.Contains(got, want) {
		t.Fatalf("updateDeps() did not update the stale requirement, want %q:\n%s", want, got)
	}
	for _, unwanted := range []string{
		"$ go get -u ./...",
		"$ go get example.com/current@",
		"$ go get example.com/third-party@",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("updateDeps() ran %q:\n%s", unwanted, got)
		}
	}
}

// TestUpdateDepsSkipsCurrentRequires checks that a module whose workspace
// requirements are all at their latest tag is reported as up to date without
// running the go tool. The module imports a package that can not be resolved
// with GOPROXY=off, so a go command that did run would report a failure.
func TestUpdateDepsSkipsCurrentRequires(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.25\n\nrequire example.com/lib v1.2.0\n")
	writeTestFile(t, filepath.Join(root, "main.go"), "package main\n\nimport _ \"example.com/lib\"\n\nfunc main() {}\n")
	t.Setenv("GOPROXY", "off")
	chdir(t, root)

	var output bytes.Buffer
	updateDeps(&output, map[string]string{"example.com/app": "."}, latestTags{"example.com/lib": "v1.2.0"}, &Options{Update: true}, false)

	want := "| Path | Module | Update status |\n" +
		"| --- | --- | --- |\n" +
		"| . | example.com/app | Already up to date. |\n"
	if got := output.String(); got != want {
		t.Fatalf("updateDeps() markdown =\n%s\nwant:\n%s", got, want)
	}
}

// TestUpdateDepsUpdateAllUpgradesEverything checks that -U still asks the go
// tool to upgrade every dependency.
func TestUpdateDepsUpdateAllUpgradesEverything(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.25\n\nrequire example.com/third-party v0.3.0\n")
	writeTestFile(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}\n")
	t.Setenv("GOPROXY", "off")
	chdir(t, root)

	var output bytes.Buffer
	updateDeps(&output, map[string]string{"example.com/app": "."}, nil, &Options{Update: true, UpdateAll: true, Verbose: true}, false)

	got := output.String()
	for _, want := range []string{"$ go get -u ./...", "$ go mod tidy"} {
		if !strings.Contains(got, want) {
			t.Fatalf("updateDeps() with -U output missing %q:\n%s", want, got)
		}
	}
}
