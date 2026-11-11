package mdfmt_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/titpetric/tools/splint/mdfmt"
)

// embedTree writes a tree of markdown and the Go that embeds some of it.
func embedTree(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for path, body := range files {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("making %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("writing %s: %v", full, err)
		}
	}
	return dir
}

// reached is the markdown Files answers, relative to the root.
func reached(t *testing.T, dir string) []string {
	t.Helper()

	found, err := mdfmt.Files(dir, true)
	if err != nil {
		t.Fatalf("Files() error = %v", err)
	}

	var got []string
	for _, path := range found {
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			t.Fatalf("Rel(): %v", err)
		}
		got = append(got, filepath.ToSlash(rel))
	}
	slices.Sort(got)
	return got
}

func TestFilesLeavesEmbeddedMarkdownAlone(t *testing.T) {
	dir := embedTree(t, map[string]string{
		"README.md":              "# prose\n",
		"docs/guide.md":          "# prose\n",
		"help/help.go":           "package help\n\nimport \"embed\"\n\n//go:embed *.md\nvar pages embed.FS\n",
		"help/one.md":            "| a | b |\n|---|---|\n",
		"help/two.md":            "| a | b |\n|---|---|\n",
		"tests/load.go":          "package tests\n\nimport \"embed\"\n\n//go:embed all:fixtures\nvar fixtures embed.FS\n",
		"tests/fixtures/x.md":    "# data\n",
		"tests/fixtures/at/y.md": "# data\n",
		"tests/own.md":           "# prose\n",
	})

	want := []string{"README.md", "docs/guide.md", "tests/own.md"}
	if got := reached(t, dir); !slices.Equal(got, want) {
		t.Errorf("Files() = %q, want %q", got, want)
	}
}

func TestFilesReadsAPatternWithAPath(t *testing.T) {
	dir := embedTree(t, map[string]string{
		"gen.go":         "package gen\n\nimport \"embed\"\n\n//go:embed templates/*.md\nvar pages embed.FS\n",
		"templates/a.md": "# data\n",
		"prose.md":       "# prose\n",
	})

	want := []string{"prose.md"}
	if got := reached(t, dir); !slices.Equal(got, want) {
		t.Errorf("Files() = %q, want %q", got, want)
	}
}

func TestFilesKeepsWhatNoDirectiveClaims(t *testing.T) {
	dir := embedTree(t, map[string]string{
		"assets.go":    "package a\n\nimport \"embed\"\n\n//go:embed *.css schema.sql\nvar files embed.FS\n",
		"site.css":     "body{}\n",
		"schema.sql":   "select 1;\n",
		"README.md":    "# prose\n",
		"other/far.go": "package other\n\nimport \"embed\"\n\n//go:embed *.md\nvar pages embed.FS\n",
		"other/x.md":   "# data\n",
		"near/y.md":    "# prose\n",
	})

	want := []string{"README.md", "near/y.md"}
	if got := reached(t, dir); !slices.Equal(got, want) {
		t.Errorf("Files() = %q, want %q", got, want)
	}
}

func TestFilesReadsADirectiveInBrokenGo(t *testing.T) {
	dir := embedTree(t, map[string]string{
		"broken.go": "package a\n\n//go:embed *.md\nvar pages embed.FS\n\nfunc F( {\n",
		"x.md":      "# data\n",
	})

	if got := reached(t, dir); len(got) != 0 {
		t.Errorf("Files() = %q, want none", got)
	}
}
