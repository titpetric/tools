package collector_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/titpetric/tools/splint/parsers/analyzer/internal/collector"
)

const exitsSource = `package x

import (
	"log"
	"os"
)

func Empty() {}

func Inline() int { return 1 }

func Falls(v int) {
	if v > 0 {
		println(v)
	}
}

func Returns(v int) error {
	if v > 0 {
		return nil
	}
	return os.ErrNotExist
}

func Mixed(v int) int {
	switch v {
	case 0:
		panic("zero")
	case 1:
		os.Exit(1)
	case 2:
		log.Fatalf("two")
	}
	return v
}

func Closure() func() int {
	f := func() int {
		return 1
	}
	return func() int {
		return f()
	}
}

func Multiline() *os.File {
	return &os.File{
	}
}
`

// TestExits counts the ways each function leaves: a return, a call that ends
// the program, and a body running off its end. A literal's return is its own.
func TestExits(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "/src/x/x.go", exitsSource, parser.ParseComments)
	require.NoError(t, err)

	info := &types.Info{Uses: map[*ast.Ident]types.Object{}, Defs: map[*ast.Ident]types.Object{}, Types: map[ast.Expr]types.TypeAndValue{}}
	pkg, err := (&types.Config{Importer: importer.Default()}).Check("example.com/x", fset, []*ast.File{file}, info)
	require.NoError(t, err)

	sink := collector.NewCollector(fset, info, pkg)
	inspector.New([]*ast.File{file}).WithStack(nil, sink.Visit)

	got := map[string]int{}
	for _, def := range sink.Clean(false) {
		for _, decl := range def.Funcs {
			got[decl.Name] = decl.Complexity.Exits
		}
	}
	assert.Equal(t, map[string]int{
		"Empty": 1, "Inline": 1, "Falls": 1, "Returns": 2,
		"Mixed": 4, "Closure": 1, "Multiline": 1,
	}, got)
}
