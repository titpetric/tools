package gofsck

import (
	"go/ast"
	"go/token"
	"path"
	"strings"

	"golang.org/x/tools/go/analysis"
)

func NewAnalyzer() *analysis.Analyzer {
	var check = &analysis.Analyzer{
		Name: "gofsck",
		Doc:  "checks for correct symbol naming in Go files, ensuring exported symbols match their filenames",
		Run:  run,
	}

	return check
}

var scanned = map[string]bool{}

// run performs the analysis logic for the Analyzer.
func run(pass *analysis.Pass) (interface{}, error) {
	var symbols []AnalyzerSymbol

	// Traverse the abstract syntax tree (AST) for each file in the package
	for _, file := range pass.Files {
		fileName := pass.Fset.Position(file.Pos()).Filename

		// No rules enforced in tests
		if strings.HasSuffix(fileName, "_test.go") {
			continue
		}

		// Only scan a file once, multiple passes are run.
		if scanned[fileName] {
			continue
		}
		scanned[fileName] = true

		if pass.Pkg.Name() == "main" {
			continue
		}

		// Collect all declared types, functions, constants, and variables
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				handleFuncDecl(pass, decl, fileName, &symbols)
			case *ast.GenDecl:
				if decl.Tok == token.TYPE {
					handleTypeDecl(pass, decl, fileName, &symbols)
				}
			}
		}
	}

	// Now that we've collected all symbols, check them
	for _, symbol := range symbols {
		matched := match(symbol, symbol.Filename)

		// If no match was found, report an error
		if !matched {
			pass.Reportf(symbol.Pos, "%s: exported %s %q does not match filename or fallback to %s", path.Base(symbol.Filename), symbol.Type, symbol.String(), symbol.Default)
		}
	}

	return nil, nil
}

// handleFuncDecl checks function declarations to ensure their names match expected filenames.
func handleFuncDecl(pass *analysis.Pass, decl *ast.FuncDecl, fileName string, symbols *[]AnalyzerSymbol) {
	// Get the receiver's name (if any) and the function name
	receiver := getReceiverName(decl)
	if receiver == "" {
		return
	}

	if !ast.IsExported(receiver) {
		return
	}

	funcName := decl.Name.Name

	// Base the default on the package name, e.g. service/service.go;
	defaultFile := path.Base(path.Dir(fileName)) + "*.go"

	// Add the symbol to the list
	*symbols = append(*symbols, AnalyzerSymbol{
		Filename: fileName,
		Symbol:   funcName,
		Receiver: receiver,
		Type:     "func",
		Default:  defaultFile,
		Pos:      decl.Pos(),
	})
}

// handleTypeDecl checks type declarations to ensure their names match expected filenames.
func handleTypeDecl(pass *analysis.Pass, decl *ast.GenDecl, fileName string, symbols *[]AnalyzerSymbol) {
	for _, spec := range decl.Specs {
		// Ensure we are working with *ast.TypeSpec
		if t, ok := spec.(*ast.TypeSpec); ok {
			// Only consider structs, skip interfaces and other types
			if _, isStruct := t.Type.(*ast.StructType); !isStruct {
				continue
			}

			typeName := t.Name.Name

			// Base the default on the package name, e.g. service/service.go;
			defaultFile := path.Base(path.Dir(fileName)) + "*.go"

			// Add the symbol to the list
			*symbols = append(*symbols, AnalyzerSymbol{
				Filename: fileName,
				Symbol:   typeName,
				Receiver: "",
				Type:     "type",
				Default:  defaultFile,
				Pos:      t.Pos(),
			})
		}
	}
}

// getReceiverName extracts the receiver's name from a function declaration (if any).
func getReceiverName(decl *ast.FuncDecl) string {
	// If there is a receiver, return its name
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		recv := decl.Recv.List[0].Type
		if starExpr, ok := recv.(*ast.StarExpr); ok {
			// If receiver is a pointer, get its underlying type name
			if ident, ok := starExpr.X.(*ast.Ident); ok {
				return ident.Name
			}
		}
		if ident, ok := recv.(*ast.Ident); ok {
			// If receiver is not a pointer, just return its name
			return ident.Name
		}
	}
	return ""
}
