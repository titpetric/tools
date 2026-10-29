package test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"strconv"
	"strings"
)

// testKind names what a function in a test file is.
type testKind int

const (
	kindFunc testKind = iota
	kindTest
	kindBenchmark
	kindFuzz
	kindExample
)

// funcDecl is one function of a file, with what a comparison and a test
// selection read of it.
type funcDecl struct {
	// Key is the receiver qualified name, the way the document keys a symbol.
	Key string
	// Name is the bare name and Receiver the base of the receiver type, if any.
	Name     string
	Receiver string

	// Signature and Body are the printed function type and block. Printing the
	// block leaves comments out, so a comment inside a body is not the change
	// this compares.
	Signature string
	Body      string

	// Refs are the identifiers and selector names the body writes, and Calls
	// the names it calls. Qualified are the calls through an imported package,
	// keyed by import path and name.
	Refs      map[string]bool
	Calls     map[string]bool
	Qualified map[string]bool

	// StartLine and EndLine are the lines the declaration spans, which a
	// coverage profile needs to place its blocks.
	StartLine int
	EndLine   int

	Kind testKind
	File string
	Dir  string
}

// fileDecls is what a parse read of one file.
type fileDecls struct {
	Package string
	Imports map[string]string // the name a file spells for an import path
	Funcs   []*funcDecl
}

// testFunc is a function a go test invocation may select.
type testFunc struct {
	Dir       string
	Name      string
	Kind      testKind
	Refs      map[string]bool
	Calls     map[string]bool
	Qualified map[string]bool

	// Imports are the import paths the file holds.
	Imports map[string]bool
}

// parseFile reads one Go file.
//
// The mode keeps comments out of the parse: the signature and block are
// printed without them, so a doc comment or a comment inside a body does not
// make a function changed.
func parseFile(path, rel, dir string) (*fileDecls, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, data, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}

	out := &fileDecls{
		Package: file.Name.Name,
		Imports: importNames(file),
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		one := &funcDecl{
			Key:       funcKey(fn),
			Name:      fn.Name.Name,
			Receiver:  receiverType(fn),
			Signature: printNode(fset, fn.Type),
			Body:      printNode(fset, fn.Body),
			Refs:      map[string]bool{},
			Calls:     map[string]bool{},
			Qualified: map[string]bool{},
			StartLine: fset.Position(fn.Pos()).Line,
			EndLine:   fset.Position(fn.End()).Line,
			File:      rel,
			Dir:       dir,
		}
		if strings.HasSuffix(path, "_test.go") {
			one.Kind = testKindOf(fn)
		}
		collectRefs(fn.Body, out.Imports, one)
		out.Funcs = append(out.Funcs, one)
	}

	return out, nil
}

// parseSource reads source a revision held, which has no file on disk now.
func parseSource(src []byte, name, rel, dir string) (*fileDecls, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}

	out := &fileDecls{Package: file.Name.Name, Imports: importNames(file)}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		out.Funcs = append(out.Funcs, &funcDecl{
			Key:       funcKey(fn),
			Name:      fn.Name.Name,
			Receiver:  receiverType(fn),
			Signature: printNode(fset, fn.Type),
			Body:      printNode(fset, fn.Body),
			StartLine: fset.Position(fn.Pos()).Line,
			EndLine:   fset.Position(fn.End()).Line,
			Kind:      kindFunc,
			File:      rel,
			Dir:       dir,
		})
	}
	return out, nil
}

// printNode renders a node as gofmt would, or "" for a function without a
// body.
func printNode(fset *token.FileSet, node ast.Node) string {
	if node == nil {
		return ""
	}
	var out strings.Builder
	if err := printer.Fprint(&out, fset, node); err != nil {
		return ""
	}
	return out.String()
}

// funcKey is the receiver qualified name, "Type.Method" or "Func".
func funcKey(fn *ast.FuncDecl) string {
	if recv := receiverType(fn); recv != "" {
		return recv + "." + fn.Name.Name
	}
	return fn.Name.Name
}

// receiverType is the base of a receiver type: the marker, the type
// parameters and any package qualifier left off.
func receiverType(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	return baseType(fn.Recv.List[0].Type)
}

// baseType reads the defined type out of a type expression.
func baseType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return baseType(t.X)
	case *ast.IndexExpr:
		return baseType(t.X)
	case *ast.IndexListExpr:
		return baseType(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	}
	return ""
}

// testKindOf reads a function in a test file.
//
// The name alone is not enough: a helper may be called TestSomething, and go
// test takes a function by the parameter the testing package gives it.
func testKindOf(fn *ast.FuncDecl) testKind {
	if fn.Recv != nil || fn.Type == nil {
		return kindFunc
	}
	name := fn.Name.Name

	switch {
	case strings.HasPrefix(name, "Test") && len(name) > len("Test") && testingParam(fn, "T"):
		return kindTest
	case strings.HasPrefix(name, "Benchmark") && len(name) > len("Benchmark") && testingParam(fn, "B"):
		return kindBenchmark
	case strings.HasPrefix(name, "Fuzz") && len(name) > len("Fuzz") && testingParam(fn, "F"):
		return kindFuzz
	case name == "Example" || strings.HasPrefix(name, "Example"):
		if fn.Type.Params.NumFields() == 0 && fn.Type.Results == nil {
			return kindExample
		}
	}
	return kindFunc
}

// testingParam reports the one parameter a test function takes: *testing.T,
// *testing.B or *testing.F.
func testingParam(fn *ast.FuncDecl, want string) bool {
	if fn.Type.Params == nil || fn.Type.Params.NumFields() != 1 {
		return false
	}
	expr := fn.Type.Params.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	sel, ok := expr.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == want
}

// importNames maps the name a file spells for an import to its path.
func importNames(file *ast.File) map[string]string {
	out := map[string]string{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := pathBase(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		out[name] = path
	}
	return out
}

// pathBase is the name a package is usually spelled by: its path's last
// element with a major version suffix left off.
func pathBase(path string) string {
	parts := strings.Split(path, "/")
	name := parts[len(parts)-1]
	if len(parts) > 1 && strings.HasPrefix(name, "v") {
		if _, err := strconv.Atoi(name[1:]); err == nil {
			return parts[len(parts)-2]
		}
	}
	return name
}

// collectRefs records what the body writes and calls.
func collectRefs(body *ast.BlockStmt, imports map[string]string, out *funcDecl) {
	if body == nil {
		return
	}

	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.Ident:
			out.Refs[n.Name] = true
		case *ast.SelectorExpr:
			out.Refs[n.Sel.Name] = true
			if x, ok := n.X.(*ast.Ident); ok {
				if path, found := imports[x.Name]; found {
					out.Qualified[path+"."+n.Sel.Name] = true
				}
			}
		case *ast.CallExpr:
			switch fun := n.Fun.(type) {
			case *ast.Ident:
				out.Calls[fun.Name] = true
			case *ast.SelectorExpr:
				out.Calls[fun.Sel.Name] = true
			}
		}
		return true
	})
}
