package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tinySourceV2 removes Do and adds Redo, so a diff of the two trees carries
// one removed and one added symbol and reads as breaking.
const tinySourceV2 = `// Package tiny is one package with one function in it.
package tiny

// Redo returns two.
func Redo() int {
	return 2
}
`

// TestDiffCommand covers the verb end to end: two trees extracted with the
// linters off, diffed as JSON, and the breaking verdict read back.
func TestDiffCommand(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "old")
	tree := writeTree(t, root, "new")
	if err := os.WriteFile(filepath.Join(tree, "tiny.go"), []byte(tinySourceV2), 0o644); err != nil {
		t.Fatal(err)
	}

	runIn(t, root, "--linters", "none", "-i", "old", "--output", "old.json", ".")
	runIn(t, root, "--linters", "none", "-i", "new", "--output", "new.json", ".")

	out := runIn(t, root, "diff", "--old", "old.json", "--new", "new.json", "--json")

	var result struct {
		Removed  []struct{ Key string } `json:"removed"`
		Added    []struct{ Key string } `json:"added"`
		Breaking bool                   `json:"breaking"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("the diff is not JSON: %v:\n%s", err, out)
	}

	if len(result.Removed) != 1 || result.Removed[0].Key != "example.com/tiny.Do" {
		t.Errorf("removed = %+v, want example.com/tiny.Do", result.Removed)
	}
	if len(result.Added) != 1 || result.Added[0].Key != "example.com/tiny.Redo" {
		t.Errorf("added = %+v, want example.com/tiny.Redo", result.Added)
	}
	if !result.Breaking {
		t.Error("a removed exported symbol is a breaking change")
	}

	rendered := runIn(t, root, "diff", "--old", "old.json", "--new", "new.json")
	for _, want := range []string{"- example.com/tiny.Do", "+ example.com/tiny.Redo", "breaking"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("the rendering is missing %q:\n%s", want, rendered)
		}
	}
}

// TestDiffRequiresBothFiles covers the refusal: a diff with one side is not a
// comparison.
func TestDiffRequiresBothFiles(t *testing.T) {
	var out bytes.Buffer
	if _, err := run(context.Background(), []string{"diff", "--old", "old.json"}, &out, &out); err == nil {
		t.Error("diff without --new did not refuse")
	}
}
