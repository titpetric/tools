package collector

import (
	"go/ast"
	"go/token"

	"github.com/fzipp/gocyclo"
	"github.com/uudashr/gocognit"

	"github.com/titpetric/tools/splint/model"
)

// complexity measures one function: how much it branches, and how long it is.
//
// Lines is the function block, from the func keyword to the brace that closes
// it. The doc comment above it is not part of the function: it was counted
// before, because the count was taken off the recorded source and the source
// carries the doc, which made a three line function that someone documented
// well read as longer than an undocumented one twice its size.
func complexity(fset *token.FileSet, in *ast.FuncDecl) *model.Complexity {
	from := fset.Position(in.Pos()).Line
	to := fset.Position(in.End()).Line

	return &model.Complexity{
		Cognitive:  gocognit.Complexity(in),
		Cyclomatic: gocyclo.Complexity(in),
		Lines:      to - from + 1,
	}
}
