package collector

import (
	"go/ast"
	"go/types"

	"github.com/titpetric/tools/splint/model"
)

// calls records what a body calls, resolved by the type checker: a package
// function as Func and a method on a named concrete type as Type.Method, keyed
// by the import path of the package declaring it. Unexported functions and the
// declaration's own file count too, because the integration rule follows a
// call wherever it goes.
//
// A call through an interface, a func value or a builtin names no
// declaration, and is not recorded.
func (v *collector) calls(fn *ast.FuncDecl) model.StringSet {
	if v.info == nil || fn == nil || fn.Body == nil {
		return nil
	}

	var found model.StringSet
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		path, symbol := v.callee(call.Fun)
		if symbol == "" {
			return true
		}
		if found == nil {
			found = model.NewStringSet()
		}
		found.Add(path, symbol)
		return true
	})
	return found
}

// callee resolves the function a call expression names, as the package path
// declaring it and the symbol, and an empty symbol for one it cannot name.
func (v *collector) callee(fun ast.Expr) (string, string) {
	fun = ast.Unparen(fun)
	switch expr := fun.(type) {
	case *ast.IndexExpr:
		fun = expr.X
	case *ast.IndexListExpr:
		fun = expr.X
	}

	var ident *ast.Ident
	switch expr := ast.Unparen(fun).(type) {
	case *ast.Ident:
		ident = expr
	case *ast.SelectorExpr:
		ident = expr.Sel
	default:
		return "", ""
	}

	obj, ok := v.info.Uses[ident].(*types.Func)
	if !ok || obj.Pkg() == nil {
		return "", ""
	}
	obj = obj.Origin()

	recv := obj.Signature().Recv()
	if recv == nil {
		return obj.Pkg().Path(), obj.Name()
	}

	named := namedReceiver(recv.Type())
	if named == nil {
		return "", ""
	}
	if _, isInterface := named.Underlying().(*types.Interface); isInterface {
		return "", ""
	}
	return obj.Pkg().Path(), named.Obj().Name() + "." + obj.Name()
}
