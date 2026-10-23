package collector

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/fzipp/gocyclo"
	"github.com/uudashr/gocognit"

	"github.com/titpetric/tools/splint/model"
)

// complexity measures one function: how much it branches, and its length.
//
// Lines is the function block, from the func keyword to the brace that closes
// it. The doc comment above the function sits outside it. An earlier count
// took the recorded source, and the source
// carries the doc, which made a three line function that someone documented
// well read as longer than an undocumented one twice its size.
func complexity(fset *token.FileSet, in *ast.FuncDecl) *model.Complexity {
	from := fset.Position(in.Pos()).Line
	to := fset.Position(in.End()).Line

	return &model.Complexity{
		Cognitive:  gocognit.Complexity(in),
		Cyclomatic: gocyclo.Complexity(in),
		Lines:      to - from + 1,
		Exits:      exits(in.Body),
	}
}

// exits counts the returns and program-ending calls of a body outside its
// function literals, plus the closing brace when the last statement is neither.
func exits(body *ast.BlockStmt) int {
	if body == nil {
		return 0
	}

	count := 0
	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			count++
		case *ast.CallExpr:
			if ends(node) {
				count++
			}
		}
		return true
	})

	if n := len(body.List); n == 0 || !terminal(body.List[n-1]) {
		count++
	}
	return count
}

// terminal reports whether a statement leaves the function.
func terminal(stmt ast.Stmt) bool {
	switch stmt := stmt.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.ExprStmt:
		call, ok := stmt.X.(*ast.CallExpr)
		return ok && ends(call)
	}
	return false
}

// ends reports whether a call ends the program: panic, os.Exit, log.Fatal*.
func ends(call *ast.CallExpr) bool {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name == "panic"
	case *ast.SelectorExpr:
		pkg, ok := fun.X.(*ast.Ident)
		if !ok {
			return false
		}
		return pkg.Name == "os" && fun.Sel.Name == "Exit" ||
			pkg.Name == "log" && strings.HasPrefix(fun.Sel.Name, "Fatal")
	}
	return false
}
