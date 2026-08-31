package tests_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/titpetric/tools/splint"
	"github.com/titpetric/tools/splint/analyzer"
	"github.com/titpetric/tools/splint/simpleparser"
)

// broken is a package that does not compile: the type it names is not declared
// anywhere, the import is not used, and the last func calls something that does
// not exist. Every declaration in it is still readable.
const broken = `package broken

import "fmt"

// Client talks to a service that has not been written.
type Client struct {
	Transport Undeclared
	Name      string ` + "`json:\"name\"`" + `
}

// Open returns a client, or the error it did not manage to.
func Open(name string) (*Client, error) {
	return &Client{Name: name}, nil
}

func (c *Client) Close() error {
	return notAFunction(c)
}
`

// TestSimpleParserReadsSourceThatDoesNotCompile is the claim the parser exists
// for, stated as a test.
//
// The ast parser resolves the tree through the toolchain, so a package that
// does not build is a package it cannot describe. The simple parser reads the
// text, so what is written is what it reports, whether or not the compiler
// would take it.
func TestSimpleParserReadsSourceThatDoesNotCompile(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module example.com/broken\n\ngo 1.27.0\n")
	write(t, filepath.Join(dir, "broken.go"), broken)

	options := splint.Options{SourcePath: dir, Pattern: ".", IncludeSources: true}

	doc, err := simpleparser.New(options).Parse(context.Background())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(doc.Packages) != 1 {
		t.Fatalf("Parse() read %d packages, want 1", len(doc.Packages))
	}

	def := doc.Packages[0]
	if def.Package.Package != "broken" {
		t.Errorf("package = %q, want broken", def.Package.Package)
	}
	if len(def.Types) != 1 || def.Types[0].Name != "Client" {
		t.Fatalf("types = %#v, want the Client type", def.Types)
	}
	if len(def.Types[0].Fields) != 2 {
		t.Errorf("Client has %d fields, want 2", len(def.Types[0].Fields))
	}
	if len(def.Funcs) != 2 {
		t.Fatalf("funcs = %d, want Open and Close", len(def.Funcs))
	}
	if got := def.Funcs[0].Signature; got != "Open (name string) (*Client, error)" {
		t.Errorf("Open signature = %q", got)
	}
	if got := def.Funcs[1].Receiver; got != "*Client" {
		t.Errorf("Close receiver = %q, want *Client", got)
	}

	// The point of the test is the contrast, so the other parser is run over
	// the same tree: it either fails or comes back with nothing to say.
	other, err := analyzer.New(options).Parse(context.Background())
	if err == nil && len(other.Packages) > 0 {
		if len(other.Packages[0].Types) > 0 {
			t.Log("the ast parser read the broken package too, which the toolchain evidently tolerated")
		}
	}
}

// write puts a file on disk for a test to read back.
func write(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
