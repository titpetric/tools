package wraphandler

import (
	"fmt"
	"go/ast"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"
)

// Analyzer performs wraphandler analysis on a set of packages.
type Analyzer struct{}

// New creates a new wraphandler analyzer.
func New() *Analyzer {
	return &Analyzer{}
}

// Analyze examines packages and returns wraphandler analysis results.
func (a *Analyzer) Analyze(pkgs []*packages.Package) (*Report, error) {
	var violations []Violation
	total := 0

	for _, pkg := range pkgs {
		if skipPackage(pkg) {
			continue
		}

		if pkg.Syntax == nil {
			continue
		}

		// Collect all unexported functions in this package for matching.
		unexported := collectUnexported(pkg)

		for _, file := range pkg.Syntax {
			filename := pkg.Fset.Position(file.Pos()).Filename
			if strings.HasSuffix(filename, "_test.go") {
				continue
			}

			for _, decl := range file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}

				if !ast.IsExported(fd.Name.Name) {
					continue
				}

				if !isHandlerFunc(fd) {
					continue
				}

				total++
				receiver := getReceiverName(fd)
				name := fd.Name.Name

				expectedName := string(unicode.ToLower(rune(name[0]))) + name[1:]
				if hasMatchingWrapper(unexported, expectedName, receiver) {
					continue
				}

				pos := pkg.Fset.Position(fd.Pos())
				violations = append(violations, Violation{
					File:     pos.Filename,
					Line:     pos.Line,
					Symbol:   name,
					Receiver: receiver,
					Message:  buildMessage(name, receiver, expectedName),
				})
			}
		}
	}

	return &Report{
		Total:      total,
		Passing:    total - len(violations),
		Violations: violations,
	}, nil
}

// skipPackage returns true if the package should be skipped.
func skipPackage(pkg *packages.Package) bool {
	if pkg.PkgPath == "main" {
		return true
	}
	if strings.HasSuffix(pkg.PkgPath, "_test") {
		return true
	}
	if strings.HasSuffix(pkg.PkgPath, ".test") {
		return true
	}
	return false
}

// unexportedFunc holds metadata about an unexported function declaration.
type unexportedFunc struct {
	name         string
	receiver     string
	returnsError bool
}

// collectUnexported gathers all unexported function declarations across the package's syntax.
func collectUnexported(pkg *packages.Package) []unexportedFunc {
	var result []unexportedFunc

	for _, file := range pkg.Syntax {
		filename := pkg.Fset.Position(file.Pos()).Filename
		if strings.HasSuffix(filename, "_test.go") {
			continue
		}

		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			if ast.IsExported(fd.Name.Name) {
				continue
			}

			result = append(result, unexportedFunc{
				name:         fd.Name.Name,
				receiver:     getReceiverName(fd),
				returnsError: returnsError(fd),
			})
		}
	}

	return result
}

// isHandlerFunc checks if a function declaration has the http.HandlerFunc signature:
// exactly 2 params (http.ResponseWriter, *http.Request) and 0 results.
func isHandlerFunc(fd *ast.FuncDecl) bool {
	if fd.Type.Results != nil && len(fd.Type.Results.List) > 0 {
		return false
	}

	if fd.Type.Params == nil {
		return false
	}

	params := fd.Type.Params.List
	// Flatten params to get individual parameter types.
	var types []ast.Expr
	for _, p := range params {
		if len(p.Names) == 0 {
			types = append(types, p.Type)
		} else {
			for range p.Names {
				types = append(types, p.Type)
			}
		}
	}

	if len(types) != 2 {
		return false
	}

	return isResponseWriter(types[0]) && isRequestPtr(types[1])
}

// isResponseWriter checks if an expression is http.ResponseWriter.
func isResponseWriter(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "http" && sel.Sel.Name == "ResponseWriter"
}

// isRequestPtr checks if an expression is *http.Request.
func isRequestPtr(expr ast.Expr) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "http" && sel.Sel.Name == "Request"
}

// returnsError checks if a function declaration returns exactly one result of type error.
func returnsError(fd *ast.FuncDecl) bool {
	if fd.Type.Results == nil {
		return false
	}
	if len(fd.Type.Results.List) != 1 {
		return false
	}
	ident, ok := fd.Type.Results.List[0].Type.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "error"
}

// getReceiverName extracts the receiver type name from a function declaration.
func getReceiverName(decl *ast.FuncDecl) string {
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		recv := decl.Recv.List[0].Type
		if starExpr, ok := recv.(*ast.StarExpr); ok {
			if ident, ok := starExpr.X.(*ast.Ident); ok {
				return ident.Name
			}
		}
		if ident, ok := recv.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

// hasMatchingWrapper checks if the unexported list contains a function with the expected name,
// matching receiver, and returning error.
func hasMatchingWrapper(funcs []unexportedFunc, expectedName string, receiver string) bool {
	for _, f := range funcs {
		if f.name == expectedName && f.receiver == receiver && f.returnsError {
			return true
		}
	}
	return false
}

// buildMessage creates the violation message.
func buildMessage(name string, receiver string, expectedName string) string {
	if receiver != "" {
		return fmt.Sprintf("%s.%s implements HandlerFunc, expected (*%s).%s(w, r) error", receiver, name, receiver, expectedName)
	}
	return fmt.Sprintf("%s implements HandlerFunc, expected %s(w, r) error", name, expectedName)
}
