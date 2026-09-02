package analyzer_test

import (
	"context"
	"os"
	"testing"

	"github.com/titpetric/tools/splint"
	"github.com/titpetric/tools/splint/analyzer"
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
	options.SourcePath = "../testdata"
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
	options.SourcePath = "../testdata/nowhere"

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
