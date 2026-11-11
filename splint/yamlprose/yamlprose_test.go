package yamlprose_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/titpetric/tools/splint/yamlprose"
)

// read writes one YAML file and answers its prose as "line: text".
func read(t *testing.T, body string) []string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "atkins.yml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("writing the file: %v", err)
	}

	file, err := yamlprose.ProseFile(dir, path)
	if err != nil {
		t.Fatalf("ProseFile() error = %v", err)
	}
	if file.Path != "atkins.yml" {
		t.Errorf("the path reads %q, want it relative to the root", file.Path)
	}

	var got []string
	for _, block := range file.Blocks {
		if block.Line == 0 {
			t.Errorf("a block names no line: %+v", block)
		}
		got = append(got, block.Text)
	}
	return got
}

func TestProseFileReadsCommentsAndDescriptions(t *testing.T) {
	got := read(t, `#!/usr/bin/env atkins
# The markdown is under data/ rather than docs/, which is why the shared
# skill does not apply here.

vars:
  folder: data

tasks:
  fmt:
    desc: "Write the markdown in one format"
    cmds:
      # --markdown keeps the fixer off the Go.
      - splint fix --markdown ./...
`)

	want := []string{
		"The markdown is under data/ rather than docs/, which is why the shared skill does not apply here.",
		"Write the markdown in one format",
		"--markdown keeps the fixer off the Go.",
	}
	if !slices.Equal(got, want) {
		t.Errorf("ProseFile() = %q, want %q", got, want)
	}
}

// A bare # ends a paragraph, the way a blank line ends one in markdown.
// Measuring a header of six paragraphs as one block would report every
// comment in the tree as enormous.
func TestProseFileSplitsACommentOnABlankLine(t *testing.T) {
	got := read(t, `# One paragraph of it.
#
# A second paragraph, which the blank line above began.

name: a
`)

	want := []string{
		"One paragraph of it.",
		"A second paragraph, which the blank line above began.",
	}
	if !slices.Equal(got, want) {
		t.Errorf("ProseFile() = %q, want %q", got, want)
	}
}

func TestProseFileReadsABlockScalar(t *testing.T) {
	got := read(t, `summary: Formats and checks markdown files.
purpose: >-
  It normalises headings and tables across the documents, and the check
  fails instead of rewriting.
notes: |
  Two commands only. The task called lint runs the check.
`)

	want := []string{
		"Formats and checks markdown files.",
		"It normalises headings and tables across the documents, and the check fails instead of rewriting.",
		"Two commands only. The task called lint runs the check.",
	}
	if !slices.Equal(got, want) {
		t.Errorf("ProseFile() = %q, want %q", got, want)
	}
}

// A file the parser rejects answers no blocks and no error: a tree under edit
// holds one. The measure over the rest still counts.
func TestProseFileSurvivesBrokenYAML(t *testing.T) {
	if got := read(t, "tasks:\n  - [unclosed\n"); len(got) != 0 {
		t.Errorf("ProseFile() = %q, want none", got)
	}
}

func TestProseFileRefusesWhatIsNotThere(t *testing.T) {
	dir := t.TempDir()
	if _, err := yamlprose.ProseFile(dir, filepath.Join(dir, "absent.yml")); err == nil {
		t.Error("ProseFile() took a path that is not there")
	}
}

// tree writes a fixture of YAML in and under a directory.
func tree(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	for _, path := range []string{
		"atkins.yml", "compose.yaml", "notes.txt", "README.md",
		"skills/go.yml", "skills/deep/docker.yml",
		"vendor/x/v.yml", "node_modules/y/n.yml", "testdata/t.yml", ".git/g.yml",
	} {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("making %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte("# A sentence with a preposition in it.\nname: x\n"), 0o644); err != nil {
			t.Fatalf("writing %s: %v", full, err)
		}
	}
	return dir
}

func TestProseWalksATree(t *testing.T) {
	dir := tree(t)

	found, err := yamlprose.Prose(dir, true)
	if err != nil {
		t.Fatalf("Prose() error = %v", err)
	}

	var got []string
	for _, file := range found {
		got = append(got, file.Path)
	}
	slices.Sort(got)

	want := []string{"atkins.yml", "compose.yaml", "skills/deep/docker.yml", "skills/go.yml"}
	if !slices.Equal(got, want) {
		t.Errorf("Prose() = %q, want %q", got, want)
	}
}

func TestFilesStopsAtOneDirectory(t *testing.T) {
	dir := tree(t)

	found, err := yamlprose.Files(dir, false)
	if err != nil {
		t.Fatalf("Files() error = %v", err)
	}
	if len(found) != 2 {
		t.Errorf("Files() = %q, want the two at the top", found)
	}
}

func TestFilesTakesOneFile(t *testing.T) {
	dir := tree(t)

	for path, want := range map[string]int{"atkins.yml": 1, "compose.yaml": 1, "notes.txt": 0} {
		found, err := yamlprose.Files(filepath.Join(dir, path), true)
		if err != nil {
			t.Fatalf("Files(%s) error = %v", path, err)
		}
		if len(found) != want {
			t.Errorf("Files(%s) = %q, want %d", path, found, want)
		}
	}
}

func TestFilesRefusesWhatIsNotThere(t *testing.T) {
	if _, err := yamlprose.Files(filepath.Join(t.TempDir(), "absent"), true); err == nil {
		t.Error("Files() took a path that is not there")
	}
}
