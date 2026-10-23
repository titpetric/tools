package collector_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/parsers/analyzer/internal/collector"
)

const callsSource = `package x

type Store struct{}

func (s *Store) Open() {}

type Opener interface{ Open() }

func helper() {}

func Run(o Opener, f func()) {
	s := &Store{}
	s.Open()
	helper()
	o.Open()
	f()
	_ = len("x")
}
`

// TestCalls records what a body calls, resolved by the type checker: a
// method on a concrete type and a function of the same file, and not a call
// through an interface, a func value or a builtin.
func TestCalls(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "/src/x/x.go", callsSource, parser.ParseComments)
	require.NoError(t, err)

	info := &types.Info{Uses: map[*ast.Ident]types.Object{}, Defs: map[*ast.Ident]types.Object{}, Types: map[ast.Expr]types.TypeAndValue{}}
	pkg, err := (&types.Config{}).Check("example.com/x", fset, []*ast.File{file}, info)
	require.NoError(t, err)

	sink := collector.NewCollector(fset, info, pkg)
	inspector.New([]*ast.File{file}).WithStack(nil, sink.Visit)

	var run *model.Declaration
	for _, def := range sink.Clean(false) {
		for _, decl := range def.Funcs {
			if decl.Name == "Run" {
				run = decl
			}
		}
	}
	require.NotNil(t, run)
	assert.Equal(t, model.StringSet{"example.com/x": {"Store.Open", "helper"}}, run.Calls)
}
