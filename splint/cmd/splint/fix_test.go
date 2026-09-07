package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/analyzer"
	settings "github.com/titpetric/tools/splint/config"
	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/simpleparser"
)

// unformatted is the fixture the import fixer is tested against, resolved
// before anything has moved the process.
var unformatted = absolute("../../linters/imports/testdata/input")

// tree writes a copy of the fixture into a directory of its own and returns
// it, so a test that rewrites source rewrites its own copy.
func tree(t *testing.T) string {
	t.Helper()

	root := filepath.Join(t.TempDir(), "tree")

	err := filepath.WalkDir(unformatted, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relative, err := filepath.Rel(unformatted, path)
		if err != nil {
			return err
		}
		target := filepath.Join(root, relative)

		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copying the fixture: %v", err)
	}

	return root
}

// TestFixCommandRewritesTheTree covers the command the prompt is about:
// splint fix writes the files and reports nothing else.
func TestFixCommandRewritesTheTree(t *testing.T) {
	dir := tree(t)

	got, code := runArgs(t, "fix", "-i", dir, "./...")

	if code != exitClean {
		t.Errorf("splint fix exited %d, want %d: a formatter that wrote files has not found a fault", code, exitClean)
	}
	for _, want := range []string{
		"fixed: main.go",
		"fixed: service2/storage/store.go",
		"left alone: broken/broken.go",
		"13 files rewritten, 1 left alone.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("splint fix did not report %q:\n%s", want, got)
		}
	}

	// The rewrite is the point, not the report.
	source := read(t, filepath.Join(dir, "service2", "storage", "store.go"))
	if !strings.Contains(source, `"example.com/imports/service2/model"`) {
		t.Errorf("model did not resolve to service2's:\n%s", source)
	}
	// The doc comment of the file names service1/model too, so the quoted
	// path is what says which one the import is.
	if strings.Contains(source, `"example.com/imports/service1/model"`) {
		t.Errorf("model resolved to the wrong service:\n%s", source)
	}
}

// TestFixCountsWhatItRewrote covers the counter, which is kept on the machine
// rather than beside the tree: a run that rewrote a file leaves the tree it
// rewrote holding only source.
func TestFixCountsWhatItRewrote(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := tree(t)

	runArgs(t, "fix", "-i", dir, "./...")

	stats, err := settings.LoadStats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.Imports.Fixed != 13 {
		t.Errorf("imports.fixed = %d, want 13", stats.Imports.Fixed)
	}

	if _, err := os.Stat(filepath.Join(dir, settings.Filename)); !os.IsNotExist(err) {
		t.Errorf("the fixer wrote %s into the tree", settings.Filename)
	}

	// A second run has nothing left to write, and the count does not move.
	got, _ := runArgs(t, "fix", "-i", dir, "./...")
	if !strings.Contains(got, "0 files rewritten") {
		t.Errorf("a second run rewrote a tree it had already written:\n%s", got)
	}

	stats, err = settings.LoadStats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.Imports.Fixed != 13 {
		t.Errorf("imports.fixed = %d after a run that wrote nothing, want 13", stats.Imports.Fixed)
	}
}

// TestFixFlagFixesThenLints covers --fix: the report is of the tree the fixer
// left, so nothing it cleared is in it.
func TestFixFlagFixesThenLints(t *testing.T) {
	dir := tree(t)

	got, code := runArgs(t, "--fix", "--linters", "imports", "-i", dir, "./...")

	if code != exitFound {
		t.Errorf("run() exited %d, want %d: broken.go still reaches a name nothing can place", code, exitFound)
	}
	if !strings.Contains(got, "fixed: main.go") {
		t.Errorf("--fix did not rewrite anything:\n%s", got)
	}
	if strings.Contains(got, "imports/format") {
		t.Errorf("--fix reported a format finding it had just cleared:\n%s", got)
	}
	if !strings.Contains(got, "imports/unresolved") {
		t.Errorf("--fix did not report the one finding it cannot clear:\n%s", got)
	}
}

// TestFixRefusesADocument covers the one combination that has no meaning: a
// document read from a file describes a tree the run never looked at.
func TestFixRefusesADocument(t *testing.T) {
	if _, err := parseOptions([]string{"fix", "--input", "splint.json"}); err == nil {
		t.Error("splint fix --input was accepted")
	}
	if _, err := parseOptions([]string{"--fix", "--input", "splint.json"}); err == nil {
		t.Error("splint --fix --input was accepted")
	}
}

// TestFixReadsWithTheQuickParser covers the parser the fixer picks for itself.
//
// A file that is missing an import it needs does not compile, and that is the
// file the fixer is there to repair. The ast parser resolves a tree through
// the toolchain, so it reads the text instead.
func TestFixReadsWithTheQuickParser(t *testing.T) {
	cfg, err := parseOptions([]string{"fix", "./..."})
	if err != nil {
		t.Fatal(err)
	}
	if got := forFixing(cfg).parser; got != simpleparser.ParserName {
		t.Errorf("the fixer read the tree with %q, want %q", got, simpleparser.ParserName)
	}

	// A parser the command line named is the one it gets, and the lint half of
	// --fix is unaffected either way.
	named, err := parseOptions([]string{"fix", "--parser", analyzer.ParserName, "./..."})
	if err != nil {
		t.Fatal(err)
	}
	if got := forFixing(named).parser; got != analyzer.ParserName {
		t.Errorf("the fixer overrode the parser it was given: %q", got)
	}
	if cfg := forFixing(named); cfg.parser != named.parser {
		t.Error("forFixing rewrote the run it was given")
	}
}

// TestBareCommandLineStillLints covers what every pipeline written before
// there were commands does.
func TestBareCommandLineStillLints(t *testing.T) {
	cfg, err := parseOptions([]string{"-i", "somewhere", "./..."})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.command != commandLint {
		t.Errorf("command = %q, want %q", cfg.command, commandLint)
	}
	if cfg.options.Pattern != "./..." {
		t.Errorf("pattern = %q", cfg.options.Pattern)
	}
}

// TestFixCommandTakesFlagsEitherWay covers the verb sitting in front of the
// flags, which is where a verb goes.
func TestFixCommandTakesFlagsEitherWay(t *testing.T) {
	cfg, err := parseOptions([]string{"fix", "./...", "-i", "somewhere"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.command != commandFix {
		t.Errorf("command = %q, want %q", cfg.command, commandFix)
	}
	if cfg.options.SourcePath != "somewhere" || cfg.options.Pattern != "./..." {
		t.Errorf("source = %q, pattern = %q", cfg.options.SourcePath, cfg.options.Pattern)
	}
}

// read returns a file the test wrote or the fixer rewrote.
func read(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestFixProgressStaysOutOfTheReport covers the two streams.
//
// --fix writes a report a program parses, and a line saying what was rewritten
// written into the middle of it is a line of invalid JSON. The progress goes
// to the second stream; splint fix, whose whole output is what it rewrote,
// writes to the first.
func TestFixProgressStaysOutOfTheReport(t *testing.T) {
	var report, progress bytes.Buffer

	if _, err := run(context.Background(),
		[]string{"--fix", "--json", "--linters", "imports", "-i", tree(t), "./..."},
		&report, &progress); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(progress.String(), "fixed: main.go") {
		t.Errorf("the fixer said nothing on the progress stream:\n%s", progress.String())
	}
	if strings.Contains(report.String(), "fixed:") {
		t.Errorf("progress leaked into the report:\n%s", report.String())
	}

	var decoded struct{ Issues []model.Issue }
	if err := json.Unmarshal(report.Bytes(), &decoded); err != nil {
		t.Fatalf("the report is not JSON: %v\n%s", err, report.String())
	}
}

// TestFixCommandWritesToTheReportStream covers the other half: splint fix has
// no report, so what it rewrote is its output.
func TestFixCommandWritesToTheReportStream(t *testing.T) {
	var report, progress bytes.Buffer

	if _, err := run(context.Background(),
		[]string{"fix", "-i", tree(t), "./..."}, &report, &progress); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(report.String(), "fixed: main.go") {
		t.Errorf("splint fix wrote nothing to its output:\n%s", report.String())
	}
	if progress.Len() != 0 {
		t.Errorf("splint fix wrote to the progress stream too:\n%s", progress.String())
	}
}
