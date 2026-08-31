package ast

import (
	"go/ast"
)

func ContainsOtherTypes(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		// It's an identifier; check if it's referring to another type/package.
		if t.Obj != nil && t.Obj.Kind == ast.Typ {
			return true // It's referring to another type/package.
		}
	case *ast.SelectorExpr:
		// It's a selector expression; check if it's referring to another package.
		// We check the X part to see if it refers to another package or type.
		if sel, ok := t.X.(*ast.Ident); ok && sel.Obj == nil {
			// If `sel.Obj` is nil, it means `sel` is referring to an imported package.
			return true
		}
		// Additionally, check if the selector part refers to a type.
		if ContainsOtherTypes(t.Sel) {
			return true
		}
	case *ast.StarExpr:
		// It's a pointer to another type.
		return ContainsOtherTypes(t.X)
	case *ast.ArrayType:
		// It's an array of another type.
		return ContainsOtherTypes(t.Elt)
	case *ast.MapType:
		// It's a map with keys and values of other types.
		return ContainsOtherTypes(t.Key) || ContainsOtherTypes(t.Value)
	case *ast.ChanType:
		// It's a channel of another type.
		return ContainsOtherTypes(t.Value)
	case *ast.FuncType:
		// It's a function type; check parameters and return types.
		for _, field := range t.Params.List {
			if ContainsOtherTypes(field.Type) {
				return true
			}
		}
		if t.Results != nil {
			for _, field := range t.Results.List {
				if ContainsOtherTypes(field.Type) {
					return true
				}
			}
		}
	case *ast.StructType:
		// It's a struct type; check field types.
		for _, field := range t.Fields.List {
			if ContainsOtherTypes(field.Type) {
				return true
			}
		}
	case *ast.InterfaceType:
		// It's an interface type; check method signatures.
		for _, field := range t.Methods.List {
			if ContainsOtherTypes(field.Type) {
				return true
			}
		}
	}

	return false
}
