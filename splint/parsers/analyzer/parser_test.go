package analyzer_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/parsers/analyzer"
	"github.com/titpetric/tools/splint/pkg/splint"
)

// TestParseRestoresTheWorkingDirectory covers what a caller still holds after a
// parse. The parse moves the process into the tree it reads, because the
// toolchain resolves a pattern from the working directory; leaving it there
// resolved every relative path the caller had a second time against that tree.
func TestParseRestoresTheWorkingDirectory(t *testing.T) {
	before, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	options := splint.NewOptions()
	options.SourcePath = "../../testdata"
	options.Pattern = "./..."

	if _, err := analyzer.New(options).Parse(context.Background()); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	after, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Errorf("Parse() left the process in %s, want %s", after, before)
	}
}

// TestParseOfANonExistentTree covers the parse that never moves anywhere: the
// directory is unchanged, so a caller reporting the failure still resolves its
// own paths.
func TestParseOfANonExistentTree(t *testing.T) {
	before, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	options := splint.NewOptions()
	options.SourcePath = "../../testdata/nowhere"

	if _, err := analyzer.New(options).Parse(context.Background()); err == nil {
		t.Fatal("Parse() read a tree that does not exist")
	}

	after, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Errorf("Parse() left the process in %s, want %s", after, before)
	}
}

// methodTree is a module built to exercise every reach the reference scan
// cannot see: a method on a local, a method on a struct field, and a method
// reached across packages.
var methodTree = map[string]string{
	"go.mod": "module example.com/tree\n\ngo 1.27.0\n",
	"x/config.go": `package x

// Config is a setting with one question to ask of it.
type Config struct{}

// Enabled answers it.
func (c Config) Enabled() bool { return true }
`,
	"x/client.go": `package x

// Client holds a config it asks.
type Client struct{ config Config }

// Open builds a client.
func Open() *Client { return &Client{} }

// Close closes it.
func (c *Client) Close() error { return nil }

// Ping asks the config through the field.
func (c *Client) Ping() bool { return c.config.Enabled() }
`,
	"x/client_test.go": `package x

import "testing"

func TestClose(t *testing.T) {
	c := Open()
	_ = c.Close()
}
`,
	"y/y.go": `package y

import "example.com/tree/x"

// Use reaches a method of another package through a local.
func Use() error {
	c := x.Open()
	return c.Close()
}
`,
}

// TestMethodReferences covers what the type checker adds to the document: a
// method call on a local or a field is recorded, on Globals for the own
// package and on References for an imported one.
func TestMethodReferences(t *testing.T) {
	dir := t.TempDir()
	for name, content := range methodTree {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	options := splint.NewOptions()
	options.SourcePath = dir
	options.Pattern = "./..."
	options.IncludeTests = true
	options.IncludeImports = true

	doc, err := analyzer.New(options).Parse(context.Background())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// A method on a struct field of the own package: Ping reaches
	// Config.Enabled with no selector naming Config anywhere.
	ping := findFunc(t, doc, "x", "Ping")
	if got := ping.Globals["Config"]; !slices.Contains(got, "Enabled") {
		t.Errorf("Ping Globals[Config] = %v, want Enabled", got)
	}

	// A method on a local of the own package, from the test half: the test
	// definition records the reach the coverage linter reads.
	testClose := findFunc(t, doc, "x_test", "TestClose")
	if got := testClose.Globals["Client"]; !slices.Contains(got, "Close") {
		t.Errorf("TestClose Globals[Client] = %v, want Close", got)
	}
	if _, ok := testClose.Globals["Open"]; !ok {
		t.Errorf("TestClose Globals = %v, want the bare Open in it", testClose.Globals)
	}

	// A method on a local of an imported package: recorded dotted, under the
	// name the file imports the package by.
	use := findFunc(t, doc, "y", "Use")
	if got := use.References["x"]; !slices.Contains(got, "Client.Close") {
		t.Errorf("Use References[x] = %v, want Client.Close", got)
	}
}

// findFunc is the one function of that name in the package of that name.
func findFunc(t *testing.T, doc *model.DocumentRoot, pkg, name string) *model.Declaration {
	t.Helper()

	for _, def := range doc.Packages {
		if def.Package.Package != pkg {
			continue
		}
		for _, decl := range def.Funcs {
			if decl.Name == name {
				return decl
			}
		}
	}

	t.Fatalf("no %s.%s in the document", pkg, name)
	return nil
}
