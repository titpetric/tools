package mdfmt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/titpetric/tools/splint/mdfmt"
)

// proseFile writes one markdown file and reads its paragraphs back.
func proseFile(t *testing.T, body string) []string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "x.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("writing the file: %v", err)
	}

	file, err := mdfmt.ProseFile(dir, path)
	if err != nil {
		t.Fatalf("ProseFile() error = %v", err)
	}
	if file.Path != "x.md" {
		t.Errorf("the path reads %q, want it relative to the root", file.Path)
	}

	var text []string
	for _, block := range file.Blocks {
		if block.Line == 0 {
			t.Errorf("a block names no line: %+v", block)
		}
		text = append(text, block.Text)
	}
	return text
}

// A fence holds code, a table holds measurements, a heading labels a section.
// None of the three is a sentence. A rule about sentences that reads them
// reports noise.
func TestProseFileReadsTheRunningTextAlone(t *testing.T) {
	got := proseFile(t, "# A heading\n\nThe frame holds\na value.\n\n```go\nfunc beside() {}\n```\n\n"+
		"| Column | Count |\n|---|---|\n| a | 1 |\n\nThe pool reuses it.\n")

	want := []string{"The frame holds a value.", "The pool reuses it."}
	if len(got) != len(want) {
		t.Fatalf("got %d blocks, want %d: %q", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("block %d reads %q, want %q", i, got[i], want[i])
		}
	}
}

func TestProseFileNumbersTheLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.md")
	if err := os.WriteFile(path, []byte("# H\n\nFirst.\n\n\nSecond\nand more.\n"), 0o644); err != nil {
		t.Fatalf("writing the file: %v", err)
	}

	file, err := mdfmt.ProseFile(dir, path)
	if err != nil {
		t.Fatalf("ProseFile() error = %v", err)
	}
	if len(file.Blocks) != 2 {
		t.Fatalf("got %d blocks, want 2: %+v", len(file.Blocks), file.Blocks)
	}
	if file.Blocks[0].Line != 3 || file.Blocks[1].Line != 6 {
		t.Errorf("the blocks start at %d and %d, want 3 and 6", file.Blocks[0].Line, file.Blocks[1].Line)
	}
}

func TestProseFileOfNothing(t *testing.T) {
	for name, body := range map[string]string{
		"headings alone": "# One\n\n## Two\n",
		"a fence alone":  "```\ncode\n```\n",
		"empty":          "",
	} {
		t.Run(name, func(t *testing.T) {
			if got := proseFile(t, body); len(got) != 0 {
				t.Errorf("got %q, want no blocks", got)
			}
		})
	}
}

func TestProseFileRefusesWhatIsNotThere(t *testing.T) {
	dir := t.TempDir()
	if _, err := mdfmt.ProseFile(dir, filepath.Join(dir, "absent.md")); err == nil {
		t.Error("ProseFile() took a path that is not there")
	}
}

func TestProseWalksATree(t *testing.T) {
	dir := t.TempDir()
	for path, body := range map[string]string{
		"top.md":       "# H\n\nThe frame holds a value.\n",
		"docs/one.md":  "# H\n\nThe pool reuses it.\n",
		"docs/bare.md": "# Headings only\n",
		"vendor/v.md":  "# H\n\nSomebody else's prose.\n",
		"notes.txt":    "The frame holds a value.",
	} {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("making %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("writing %s: %v", full, err)
		}
	}

	found, err := mdfmt.Prose(dir, true)
	if err != nil {
		t.Fatalf("Prose() error = %v", err)
	}

	if len(found) != 2 {
		var paths []string
		for _, file := range found {
			paths = append(paths, file.Path)
		}
		t.Fatalf("got %d files, want 2: %q", len(found), paths)
	}
	for _, file := range found {
		if len(file.Blocks) == 0 {
			t.Errorf("%s is in the answer with no blocks", file.Path)
		}
	}
}

func TestProseRefusesWhatIsNotThere(t *testing.T) {
	if _, err := mdfmt.Prose(filepath.Join(t.TempDir(), "absent"), true); err == nil {
		t.Error("Prose() took a path that is not there")
	}
}
