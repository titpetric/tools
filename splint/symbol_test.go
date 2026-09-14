package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// countedModule is the tiny module with a type, a method and a test, so the
// symbol rendering has something reached and something not.
var countedModule = map[string]string{
	"go.mod": `module example.com/tiny

go 1.27.0
`,
	"tiny.go": `// Package tiny is one package with a counter in it.
package tiny

// Counter counts.
type Counter struct{ n int }

// NewCounter builds a counter.
func NewCounter() *Counter { return &Counter{} }

// Add counts n in.
func (c *Counter) Add(n int) { c.n += n }

// Reset starts the count over.
func (c *Counter) Reset() { c.n = 0 }
`,
	"tiny_test.go": `package tiny

import "testing"

func TestAdd(t *testing.T) {
	c := NewCounter()
	c.Add(1)
}
`,
}

// writeCounted writes the module into a new directory under root.
func writeCounted(t *testing.T, root string) {
	t.Helper()

	for name, content := range countedModule {
		path := filepath.Join(root, "tree", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestDocsSymbol covers the symbol rendering: the declaration godoc style,
// and how many tests reach it.
func TestDocsSymbol(t *testing.T) {
	root := t.TempDir()
	writeCounted(t, root)

	out := runIn(t, root, "docs", "-i", "tree", ".", "Counter.Add")

	for _, want := range []string{
		`package tiny // import "example.com/tiny"`,
		"func (*Counter) Add",
		"Add counts n in.",
		"Tested by 1 test.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the symbol page is missing %q:\n%s", want, out)
		}
	}
}

// TestDocsSymbolUntested covers the count at zero: a method nothing reaches
// says so.
func TestDocsSymbolUntested(t *testing.T) {
	root := t.TempDir()
	writeCounted(t, root)

	out := runIn(t, root, "docs", "-i", "tree", ".", "Counter.Reset")

	if !strings.Contains(out, "Tested by 0 tests.") {
		t.Errorf("an unreached method does not say so:\n%s", out)
	}
}

// TestDocsSymbolVerbose covers the reference listing: one line per reference,
// the tests marked as tests, at the position of the referencing declaration.
func TestDocsSymbolVerbose(t *testing.T) {
	root := t.TempDir()
	writeCounted(t, root)

	out := runIn(t, root, "docs", "-v", "-i", "tree", ".", "NewCounter")

	for _, want := range []string{"References:", "test", "TestAdd", "tiny_test.go:5"} {
		if !strings.Contains(out, want) {
			t.Errorf("the verbose page is missing %q:\n%s", want, out)
		}
	}
}

// TestDocsSymbolMissing covers the error: a symbol the document does not hold
// is said to be missing rather than rendered as nothing.
func TestDocsSymbolMissing(t *testing.T) {
	root := t.TempDir()
	writeCounted(t, root)
	t.Chdir(root)

	var out bytes.Buffer
	_, err := run(context.Background(), []string{"docs", "-i", "tree", ".", "Nowhere"}, &out, &out)
	if err == nil || !strings.Contains(err.Error(), "Nowhere") {
		t.Errorf("run() error = %v, want the symbol named", err)
	}
}
