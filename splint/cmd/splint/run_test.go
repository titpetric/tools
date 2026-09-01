package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/model"
)

// fixture is the module of code that fails checks on purpose, which is what
// gives these tests something to find.
//
// It is resolved once, and absolute: listing packages leaves the process in
// the directory it read, so a relative path stops meaning the same thing after
// the first parse.
var fixture = absolute("../../testdata")

// absolute resolves a path at startup, before anything has moved.
func absolute(path string) string {
	out, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return out
}

// runArgs runs the command as the shell would and returns what it wrote.
func runArgs(t *testing.T, args ...string) (string, int) {
	t.Helper()

	var out bytes.Buffer
	code, err := run(context.Background(), args, &out)
	if err != nil {
		t.Fatalf("run(%v) error = %v", args, err)
	}
	return out.String(), code
}

// TestRunReportsTheFixture covers the whole of a plain run: the tree is
// parsed, every linter reads it, and what they found is written out.
func TestRunReportsTheFixture(t *testing.T) {
	got, code := runArgs(t, "-i", fixture, "./...")

	if code != exitFound {
		t.Errorf("run() exited %d, want %d: a fixture that fails checks has to fail them", code, exitFound)
	}
	// One line per finding, opening on the level and naming the file the way
	// a compiler does.
	if !strings.Contains(got, "WARN: undocumented.go:9: godoc/missing: Undocumented - ") {
		t.Fatalf("run() wrote no findings:\n%s", got)
	}
	if strings.Contains(got, "\033") {
		t.Error("run() wrote escape codes to something that is not a terminal")
	}
}

// TestRunJSON covers the flag that skips the rendering: the same findings,
// for a reader that is a program.
func TestRunJSON(t *testing.T) {
	got, code := runArgs(t, "-json", "-i", fixture, "./...")

	if code != exitFound {
		t.Errorf("run() exited %d, want %d", code, exitFound)
	}

	var found struct {
		Issues  []model.Issue
		Linters []string
	}
	if err := json.Unmarshal([]byte(got), &found); err != nil {
		t.Fatalf("-json wrote something that is not JSON: %v\n%s", err, got)
	}
	if len(found.Issues) == 0 || len(found.Linters) == 0 {
		t.Fatalf("-json wrote %d issues from %d linters", len(found.Issues), len(found.Linters))
	}
	for _, issue := range found.Issues {
		if issue.Linter == "" || issue.Message == "" {
			t.Errorf("an issue is missing what it says: %#v", issue)
		}
	}
}

// TestRunStatsJSON covers the measurements as data: the numbers behind the
// tables rather than the tables.
func TestRunStatsJSON(t *testing.T) {
	got, code := runArgs(t, "-json", "-stats", "-i", fixture, "./...")

	if code != exitClean {
		t.Errorf("run() exited %d, want %d", code, exitClean)
	}

	var measured []struct {
		Linter     string
		Metrics    model.LintMetrics
		Statistics []model.Statistics
	}
	if err := json.Unmarshal([]byte(got), &measured); err != nil {
		t.Fatalf("-json -stats wrote something that is not JSON: %v\n%s", err, got)
	}
	if len(measured) == 0 {
		t.Fatal("-json -stats wrote nothing")
	}

	for _, one := range measured {
		if one.Linter == "godoc" {
			if one.Metrics.Empty() || len(one.Statistics) == 0 {
				t.Errorf("godoc measured nothing: %#v", one)
			}
			return
		}
	}
	t.Error("-json -stats did not write godoc")
}

// TestRunStats covers the flag that asks what the linters measured rather than
// what they found.
func TestRunStats(t *testing.T) {
	got, code := runArgs(t, "-stats", "-i", fixture, "./...")

	// Statistics are not findings, so a run asking for them is not failing.
	if code != exitClean {
		t.Errorf("run() exited %d, want %d", code, exitClean)
	}
	if strings.Contains(got, "| Position") {
		t.Errorf("-stats wrote the issue table:\n%s", got)
	}
	if !strings.Contains(got, "|--") {
		t.Fatalf("-stats wrote no table:\n%s", got)
	}
	// One blank line between tables and no more.
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("-stats left more than one blank line between tables:\n%q", got)
	}
}

func TestRunParsers(t *testing.T) {
	ast, _ := runArgs(t, "-i", fixture, "--parser=astparser", "./...")
	simple, _ := runArgs(t, "-i", fixture, "--parser=simpleparser", "./...")

	// The two parsers produce the same document, so the linters reading it
	// have to reach the same conclusions.
	if ast != simple {
		t.Errorf("the parsers disagree about the fixture:\n--- ast ---\n%s\n--- simple ---\n%s", ast, simple)
	}
}

func TestRunUnknownParser(t *testing.T) {
	var out bytes.Buffer
	if _, err := run(context.Background(), []string{"--parser=nope", "-i", fixture, "./..."}, &out); err == nil {
		t.Fatal("run() accepted a parser that does not exist")
	} else if !strings.Contains(err.Error(), "astparser") {
		t.Errorf("run() error = %v, want the parsers it does have", err)
	}
}

func TestRunUnknownLinter(t *testing.T) {
	var out bytes.Buffer
	_, err := run(context.Background(), []string{"--linters", "nope", "-i", fixture, "./..."}, &out)
	if err == nil {
		t.Fatal("run() accepted a linter that does not exist")
	}
	// A typo in a flag is a linter silently not running, so it is reported.
	if !strings.Contains(err.Error(), "nope") || !strings.Contains(err.Error(), "godoc") {
		t.Errorf("run() error = %v", err)
	}
}

// TestRunOutputAndInput covers the two halves of reading a document back: a
// run writes one, and another run lints it without parsing anything.
func TestRunOutputAndInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.json")

	parsed, _ := runArgs(t, "-i", fixture, "--output", path, "./...")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("--output wrote nothing: %v", err)
	}

	loaded, _ := runArgs(t, "--input", path)
	if parsed != loaded {
		t.Errorf("a document read back reports differently:\n--- parsed ---\n%s\n--- loaded ---\n%s", parsed, loaded)
	}
}

func TestRunSchema(t *testing.T) {
	got, code := runArgs(t, "-schema", "-i", fixture, "./...")

	if code != exitClean {
		t.Errorf("run() exited %d", code)
	}

	var schema struct {
		Schema      string         `json:"$schema"`
		Definitions map[string]any `json:"definitions"`
	}
	if err := json.Unmarshal([]byte(got), &schema); err != nil {
		t.Fatalf("-schema wrote something that is not JSON: %v\n%s", err, got)
	}
	if !strings.Contains(schema.Schema, "json-schema.org") {
		t.Errorf("$schema = %q", schema.Schema)
	}
	// The fixture declares a User and a Thing, so the schema describes them.
	for _, want := range []string{"User", "Thing"} {
		if _, ok := schema.Definitions[want]; !ok {
			t.Errorf("definitions have no %q: %v", want, keys(schema.Definitions))
		}
	}
}

func TestRunHelp(t *testing.T) {
	got, code := runArgs(t, "--help")

	if code != exitClean {
		t.Errorf("run() exited %d", code)
	}
	for _, want := range []string{"-parser NAME", "-stats", "-schema", "astparser", "simpleparser"} {
		if !strings.Contains(got, want) {
			t.Errorf("--help does not mention %q", want)
		}
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	return out
}
