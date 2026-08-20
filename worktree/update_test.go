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

func TestUpdateDepsReportsFailures(t *testing.T) {
	chdir(t, t.TempDir())

	var output bytes.Buffer
	updateDeps(&output, map[string]string{"example.com/missing": "./missing"}, nil, &Options{Update: true}, false)

	got := output.String()
	if strings.Contains(got, "Already up to date.") {
		t.Fatalf("updateDeps() reported success for a missing module:\n%s", got)
	}
	for _, want := range []string{
		"failed to read go.mod",
		"go get -u ./...:",
		"go mod tidy:",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("updateDeps() output missing %q:\n%s", want, got)
		}
	}
}
