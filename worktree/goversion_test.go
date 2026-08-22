package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/titpetric/tools/worktree/config"
)

func TestParseGoVersion(t *testing.T) {
	valid := map[string]string{
		"1.27":     "1.27",
		"go1.27":   "1.27",
		" 1.27.1 ": "1.27.1",
		"1.28rc1":  "1.28rc1",
	}
	for input, want := range valid {
		got, err := parseGoVersion(input)
		if err != nil {
			t.Fatalf("parseGoVersion(%q) error: %v", input, err)
		}
		if got != want {
			t.Fatalf("parseGoVersion(%q) = %q, want %q", input, got, want)
		}
	}

	for _, input := range []string{"", "1", "v1.27", "1.27.", "latest"} {
		if got, err := parseGoVersion(input); err == nil {
			t.Fatalf("parseGoVersion(%q) = %q, want an error", input, got)
		}
	}
}

func TestGoVersionChange(t *testing.T) {
	if got, want := goVersionChange("1.25", "1.27"), "go 1.25 → 1.27"; got != want {
		t.Fatalf("goVersionChange() = %q, want %q", got, want)
	}
	if got, want := goVersionChange("", "1.27"), "+ go 1.27"; got != want {
		t.Fatalf("goVersionChange() = %q, want %q", got, want)
	}
}

func TestSetGoVersion(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "go.mod")
	writeTestFile(t, path, "module example.com/app\n\ngo 1.25\n\nrequire example.com/lib v1.0.0\n")

	before, err := setGoVersion(root, "1.27")
	if err != nil {
		t.Fatalf("setGoVersion() error: %v", err)
	}
	if before != "1.25" {
		t.Fatalf("setGoVersion() = %q, want %q", before, "1.25")
	}

	got := readTestFile(t, path)
	if !strings.Contains(got, "go 1.27\n") {
		t.Fatalf("go.mod missing the new go directive:\n%s", got)
	}
	if !strings.Contains(got, "require example.com/lib v1.0.0") {
		t.Fatalf("go.mod lost its requirements:\n%s", got)
	}

	// A module already at the version is left alone.
	if before, err = setGoVersion(root, "1.27"); err != nil || before != "1.27" {
		t.Fatalf("setGoVersion() = %q, %v, want %q, nil", before, err, "1.27")
	}
}

func TestSetGoVersionDropsStaleToolchain(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "go.mod")
	writeTestFile(t, path, "module example.com/app\n\ngo 1.25\n\ntoolchain go1.25.3\n")

	if _, err := setGoVersion(root, "1.27"); err != nil {
		t.Fatalf("setGoVersion() error: %v", err)
	}
	if got := readTestFile(t, path); strings.Contains(got, "toolchain") {
		t.Fatalf("go.mod kept a toolchain older than the go directive:\n%s", got)
	}

	// A toolchain newer than the go directive is valid and kept.
	writeTestFile(t, path, "module example.com/app\n\ngo 1.25\n\ntoolchain go1.28.1\n")
	if _, err := setGoVersion(root, "1.27"); err != nil {
		t.Fatalf("setGoVersion() error: %v", err)
	}
	if got := readTestFile(t, path); !strings.Contains(got, "toolchain go1.28.1") {
		t.Fatalf("go.mod dropped a newer toolchain:\n%s", got)
	}
}

func TestSetGoVersionMissingModule(t *testing.T) {
	if _, err := setGoVersion(t.TempDir(), "1.27"); err == nil {
		t.Fatal("setGoVersion() without a go.mod, want an error")
	}
}

func TestSetGoWorkVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go.work")
	writeTestFile(t, path, "go 1.25\n\nuse (\n\t./app\n\t./lib\n)\n")

	before, err := setGoWorkVersion(path, "1.27")
	if err != nil {
		t.Fatalf("setGoWorkVersion() error: %v", err)
	}
	if before != "1.25" {
		t.Fatalf("setGoWorkVersion() = %q, want %q", before, "1.25")
	}

	got := readTestFile(t, path)
	if !strings.Contains(got, "go 1.27\n") {
		t.Fatalf("go.work missing the new go directive:\n%s", got)
	}
	if !strings.Contains(got, "./lib") {
		t.Fatalf("go.work lost its use directives:\n%s", got)
	}
}

func TestFindGoWorkFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.work"), "go 1.25\n\nuse ./app\n")
	writeTestFile(t, filepath.Join(root, "nested", "go.work"), "go 1.25\n\nuse .\n")
	writeTestFile(t, filepath.Join(root, "app", "go.mod"), "module example.com/app\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, ".git", "go.work"), "go 1.25\n")
	chdir(t, root)

	want := []string{"go.work", filepath.Join("nested", "go.work")}
	if got := findGoWorkFiles(".", config.Default().Scan); !reflect.DeepEqual(got, want) {
		t.Fatalf("findGoWorkFiles() = %v, want %v", got, want)
	}
}

func TestUpdateGoWorkVersions(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.work"), "go 1.25\n\nuse ./app\n")
	writeTestFile(t, filepath.Join(root, "app", "go.mod"), "module example.com/app\n\ngo 1.25\n")
	chdir(t, root)

	var output bytes.Buffer
	if err := updateGoWorkVersions(&output, ".", "1.27", config.Default().Scan, false); err != nil {
		t.Fatalf("updateGoWorkVersions() error: %v", err)
	}
	if got, want := output.String(), "./go.work: go 1.25 → 1.27\n"; got != want {
		t.Fatalf("updateGoWorkVersions() = %q, want %q", got, want)
	}

	// A second run has nothing to report.
	output.Reset()
	if err := updateGoWorkVersions(&output, ".", "1.27", config.Default().Scan, false); err != nil {
		t.Fatalf("updateGoWorkVersions() error: %v", err)
	}
	if got := output.String(); got != "" {
		t.Fatalf("updateGoWorkVersions() = %q, want no output", got)
	}
}

func TestUpdateDepsSetsGoVersion(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}\n")
	chdir(t, root)

	var output bytes.Buffer
	updateDeps(&output, map[string]string{"example.com/app": "."}, nil, &Options{GoVersion: "1.26"}, false)

	if got, want := output.String(), "| . | example.com/app | go 1.25 → 1.26 |\n"; !strings.Contains(got, want) {
		t.Fatalf("updateDeps() markdown =\n%s\nwant a row %s", got, want)
	}
	if got := readTestFile(t, filepath.Join(root, "go.mod")); !strings.Contains(got, "go 1.26") {
		t.Fatalf("go.mod not updated:\n%s", got)
	}
}

// TestUpdateDepsSkipsMatchingGoVersion checks that a module already declaring
// the requested version is reported as up to date without running the go tool.
// The module imports a package that can not be resolved with GOPROXY=off, so a
// go command that did run would report a failure.
func TestUpdateDepsSkipsMatchingGoVersion(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "go.mod")
	writeTestFile(t, path, "module example.com/app\n\ngo 1.26\n")
	writeTestFile(t, filepath.Join(root, "main.go"), "package main\n\nimport _ \"example.com/lib\"\n\nfunc main() {}\n")
	t.Setenv("GOPROXY", "off")
	chdir(t, root)

	var output bytes.Buffer
	updateDeps(&output, map[string]string{"example.com/app": "."}, nil, &Options{GoVersion: "1.26"}, false)

	want := "| Path | Module | Update status |\n" +
		"| --- | --- | --- |\n" +
		"| . | example.com/app | Already up to date. |\n"
	if got := output.String(); got != want {
		t.Fatalf("updateDeps() markdown =\n%s\nwant:\n%s", got, want)
	}
	if got, want := readTestFile(t, path), "module example.com/app\n\ngo 1.26\n"; got != want {
		t.Fatalf("go.mod rewritten:\n%s\nwant:\n%s", got, want)
	}

	// With -u the module's stale requirements are updated regardless of its
	// go directive.
	writeTestFile(t, path, "module example.com/app\n\ngo 1.26\n\nrequire example.com/lib v1.0.0\n")
	output.Reset()
	tags := latestTags{"example.com/lib": "v1.2.0"}
	updateDeps(&output, map[string]string{"example.com/app": "."}, tags, &Options{GoVersion: "1.26", Update: true}, false)
	if got, want := output.String(), "go get example.com/lib@v1.2.0:"; !strings.Contains(got, want) {
		t.Fatalf("updateDeps() with -u skipped the update:\n%s", got)
	}
}

// TestUpdateWorkspaceGoVersion runs the go.work and go.mod updates the way
// main does, over a workspace of two modules.
func TestUpdateWorkspaceGoVersion(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.work"), "go 1.24\n\nuse (\n\t./app\n\t./lib\n)\n")
	writeTestFile(t, filepath.Join(root, "app", "go.mod"), "module example.com/app\n\ngo 1.24\n\ntoolchain go1.24.1\n")
	writeTestFile(t, filepath.Join(root, "app", "main.go"), "package main\n\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(root, "lib", "go.mod"), "module example.com/lib\n\ngo 1.24\n")
	writeTestFile(t, filepath.Join(root, "lib", "lib.go"), "package lib\n")
	chdir(t, root)

	var output bytes.Buffer
	if err := updateGoWorkVersions(&output, ".", "1.25", config.Default().Scan, false); err != nil {
		t.Fatalf("updateGoWorkVersions() error: %v", err)
	}
	modPaths := map[string]string{"example.com/app": "./app", "example.com/lib": "./lib"}
	updateDeps(&output, modPaths, nil, &Options{GoVersion: "1.25"}, false)

	got := output.String()
	for _, want := range []string{
		"./go.work: go 1.24 → 1.25\n",
		"| ./app | example.com/app | go 1.24 → 1.25 |",
		"| ./lib | example.com/lib | go 1.24 → 1.25 |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("workspace update output missing %q:\n%s", want, got)
		}
	}

	for path, want := range map[string]string{
		"go.work":     "go 1.25\n",
		"app/go.mod":  "go 1.25\n",
		"lib/go.mod":  "go 1.25\n",
		"app/main.go": "func main() {}\n",
	} {
		if content := readTestFile(t, filepath.Join(root, path)); !strings.Contains(content, want) {
			t.Fatalf("%s missing %q:\n%s", path, want, content)
		}
	}
	if content := readTestFile(t, filepath.Join(root, "app", "go.mod")); strings.Contains(content, "toolchain go1.24.1") {
		t.Fatalf("app/go.mod kept a toolchain older than the go directive:\n%s", content)
	}
}

func TestParseOptionsGoVersion(t *testing.T) {
	args := map[string][]string{
		"--go=go1.27": {"worktree", "--go=go1.27"},
		"--go 1.27":   {"worktree", "--go", "1.27", "./..."},
		"-go 1.27 -v": {"worktree", "-go", "1.27", "-v"},
	}
	for name, argv := range args {
		originalArgs := os.Args
		originalFlags := flag.CommandLine
		os.Args = argv
		flag.CommandLine = flag.NewFlagSet("worktree", flag.ContinueOnError)
		flag.CommandLine.SetOutput(io.Discard)

		opts := ParseOptions()
		os.Args = originalArgs
		flag.CommandLine = originalFlags

		if opts.GoVersion != "1.27" {
			t.Fatalf("ParseOptions(%s) GoVersion = %q, want %q", name, opts.GoVersion, "1.27")
		}
		if opts.FilterArg != "" {
			t.Fatalf("ParseOptions(%s) treated the version as a filter: %#v", name, opts)
		}
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
