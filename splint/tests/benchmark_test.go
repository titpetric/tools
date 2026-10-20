package tests_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/titpetric/tools/splint/parsers/analyzer"
	"github.com/titpetric/tools/splint/parsers/simpleparser"
	"github.com/titpetric/tools/splint/pkg/splint"
)

// benchProject is the tree the benchmarks read. This tree is the largest one
// to hand, so the parse dominates a profile taken over it and the process
// start-up does not.
//
// SPLINT_BENCH names another.
var benchProject = envOr("SPLINT_BENCH", "phpscript")

// Each parser has a benchmark of its own and no shared benchmark with two
// cases exists, so a profile can cover one of them: a run covering both mixes
// the two into a profile that describes neither.

func BenchmarkSimpleParser(b *testing.B) {
	benchmark(b, func(options splint.Options) splint.Parser {
		return simpleparser.New(options)
	})
}

func BenchmarkASTParser(b *testing.B) {
	benchmark(b, func(options splint.Options) splint.Parser {
		return analyzer.New(options)
	})
}

// benchmark reads the project once per iteration, sources and all, which is
// the heaviest job either parser gets.
func benchmark(b *testing.B, parser func(splint.Options) splint.Parser) {
	b.Helper()

	root := filepath.Join(workspace, benchProject)
	if _, err := os.Stat(root); err != nil {
		b.Skipf("%s is not checked out", benchProject)
	}

	options := splint.Options{SourcePath: root, Pattern: "./...", IncludeSources: true}
	ctx := context.Background()

	b.ReportAllocs()
	for b.Loop() {
		doc, err := parser(options).Parse(ctx)
		if err != nil {
			b.Fatal(err)
		}
		if len(doc.Packages) == 0 {
			b.Fatal("the parse read nothing")
		}
	}
}
