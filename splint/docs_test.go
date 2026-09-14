package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDocsCommand covers the reference end to end: the tree is parsed with
// sources whether or not they were asked for, and the markdown holds the
// declaration.
func TestDocsCommand(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "tree")

	out := runIn(t, root, "docs", "-i", "tree", ".")

	for _, want := range []string{"# Package tiny", "example.com/tiny", "Do returns one.", "func Do () int"} {
		if !strings.Contains(out, want) {
			t.Errorf("the reference is missing %q:\n%s", want, out)
		}
	}
}

// TestDocsSplit covers split mode: one file per package under --out, and a
// README listing them.
func TestDocsSplit(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "tree")

	runIn(t, root, "docs", "-i", "tree", "--split", "--out", "api", "--strip-prefix", "example.com", ".")

	for _, name := range []string{"README.md", "tiny.md"} {
		if _, err := os.Stat(filepath.Join(root, "api", name)); err != nil {
			t.Errorf("--split wrote no %s: %v", name, err)
		}
	}
}

// TestCoverageCommand covers the report end to end: the profile is folded
// into the parse and the report names the function with its coverage.
func TestCoverageCommand(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "tree")

	if err := os.WriteFile(filepath.Join(root, "pkg.cov"), []byte(tinyProfile), 0o644); err != nil {
		t.Fatal(err)
	}

	out := runIn(t, root, "coverage", "-i", "tree", "--append-coverage=pkg.cov", ".")

	for _, want := range []string{"| Do", "100.00%"} {
		if !strings.Contains(out, want) {
			t.Errorf("the report is missing %q:\n%s", want, out)
		}
	}
}

// TestCoverageTemplate covers --template: the page is the template with the
// two tables in it rather than the function table alone.
func TestCoverageTemplate(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "tree")

	for file, content := range map[string]string{
		"pkg.cov":     tinyProfile,
		"page.md.tpl": "# Coverage\n\n{{.Packages}}\n\n{{.Functions}}\n",
	} {
		if err := os.WriteFile(filepath.Join(root, file), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	out := runIn(t, root, "coverage", "-i", "tree", "--append-coverage=pkg.cov", "--template=page.md.tpl", ".")

	for _, want := range []string{"# Coverage", "| Do", "| Lines", "100.00%"} {
		if !strings.Contains(out, want) {
			t.Errorf("the page is missing %q:\n%s", want, out)
		}
	}
}
