package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

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
	// Redirected output is for a program: one workflow command per finding,
	// which is what GitHub Actions turns into an annotation on a review.
	if !strings.Contains(got, "::warning file=undocumented.go,line=9,title=godoc/missing::Undocumented - ") {
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

// TestRunFlagsAfterThePattern covers the command line written the way a person
// writes it: the flag package stops at the first operand, so a flag after the
// pattern used to be read as a second pattern and the run was silently not the
// one that was asked for.
func TestRunFlagsAfterThePattern(t *testing.T) {
	before, code := runArgs(t, "-i", fixture, "--linters", "godoc", "./...")
	if code != exitFound {
		t.Fatalf("run() exited %d with the flags first", code)
	}

	after, code := runArgs(t, "./...", "-i", fixture, "--linters", "godoc")
	if code != exitFound {
		t.Errorf("run() exited %d with the flags after the pattern", code)
	}
	if after != before {
		t.Errorf("the flags after the pattern read a different run:\n%s\n---\n%s", after, before)
	}
}

// TestReorder covers what carries a value and what does not, which is what the
// flag set is asked rather than guessed.
func TestReorder(t *testing.T) {
	fs := flag.NewFlagSet("splint", flag.ContinueOnError)
	fs.String("i", "", "")
	fs.String("linters", "", "")
	fs.Bool("json", false, "")

	tests := []struct {
		args []string
		want []string
	}{
		// A bool takes nothing with it, and the operand goes last.
		{[]string{"./...", "-json"}, []string{"-json", "./..."}},
		// A flag written as two words takes the second with it.
		{[]string{"./...", "-i", "path"}, []string{"-i", "path", "./..."}},
		{[]string{"./...", "-i=path"}, []string{"-i=path", "./..."}},
		// Already in order, and left that way.
		{[]string{"-json", "./..."}, []string{"-json", "./..."}},
		// Everything after a bare -- is an operand.
		{[]string{"--", "-json"}, []string{"-json"}},
		{[]string{"-linters", "godoc", "./...", "-json"}, []string{"-linters", "godoc", "-json", "./..."}},
	}

	for _, test := range tests {
		got := reorder(fs, test.args)
		if len(got) != len(test.want) {
			t.Errorf("reorder(%v) = %v, want %v", test.args, got, test.want)
			continue
		}
		for i := range got {
			if got[i] != test.want[i] {
				t.Errorf("reorder(%v) = %v, want %v", test.args, got, test.want)
				break
			}
		}
	}
}

// TestRunYAML covers the second encoding: the same answer, written the way a
// document is written.
func TestRunYAML(t *testing.T) {
	got, code := runArgs(t, "-yaml", "-i", fixture, "./...")

	if code != exitFound {
		t.Errorf("run() exited %d, want %d", code, exitFound)
	}

	var found struct {
		Issues  []model.Issue `yaml:"Issues"`
		Linters []string      `yaml:"Linters"`
	}
	if err := yaml.Unmarshal([]byte(got), &found); err != nil {
		t.Fatalf("-yaml wrote something that is not YAML: %v\n%s", err, got)
	}
	if len(found.Issues) == 0 {
		t.Fatalf("-yaml wrote no issues:\n%s", got)
	}

	// A severity is a level, and it reads as the word in both encodings.
	if !strings.Contains(got, "Severity: WARN") {
		t.Errorf("-yaml wrote a severity as something other than the word:\n%s", got)
	}
}

// TestRunOneEncoding covers asking for both: they are two encodings of one
// answer, and a run cannot write both to one stream.
func TestRunOneEncoding(t *testing.T) {
	var out bytes.Buffer
	_, err := run(context.Background(), []string{"-json", "-yaml", "-i", fixture, "./..."}, &out)
	if err == nil {
		t.Fatal("run() accepted both encodings")
	}
	if !strings.Contains(err.Error(), "-json") || !strings.Contains(err.Error(), "-yaml") {
		t.Errorf("run() error = %v", err)
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
