package grouping

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
	"sync"

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

var (
	scanned   = map[string]bool{}
	scannedMu sync.Mutex
)

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

		if pass.Pkg.Name() == "main" {
			continue
		}

		// Only scan a file once, multiple passes are run.
		scannedMu.Lock()
		if scanned[fileName] {
			scannedMu.Unlock()
			continue
		} else {
			scanned[fileName] = true
			scannedMu.Unlock()
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
		matched, canonicalLocations, totalExpected := matchWithOptions(symbol, symbol.Filename)

		// If no match was found, report an error
		if !matched {
			locStr := strings.Join(canonicalLocations, ", ")
			pass.Reportf(symbol.Pos, "exported %s %q expected in [%s] (total: %d expected filenames)", symbol.Type, symbol.String(), locStr, totalExpected)
		}
	}

	return nil, nil
}

// handleFuncDecl checks function declarations to ensure their names match expected filenames.
func handleFuncDecl(pass *analysis.Pass, decl *ast.FuncDecl, fileName string, symbols *[]AnalyzerSymbol) {
	packageName := pass.Pkg.Name()
	funcName := decl.Name.Name

	// If the function is not exported, skip it
	if !ast.IsExported(funcName) {
		return
	}

	// Base the default on the package name, e.g. service/service.go;
	defaultFile := packageName + "*.go"

	// Get the receiver's name (if any) and the function name
	receiver, isInterface := getReceiverNameAndType(pass, decl)
	if receiver != "" {
		// Skip methods on interfaces
		if isInterface {
			return
		}

		if !ast.IsExported(receiver) {
			return
		}

		// Add the symbol to the list for methods
		*symbols = append(*symbols, AnalyzerSymbol{
			Package:  packageName,
			Filename: fileName,
			Symbol:   funcName,
			Receiver: receiver,
			Type:     "func",
			Default:  defaultFile,
			Pos:      decl.Pos(),
		})
	} else if decl.Type.Results != nil && len(decl.Type.Results.List) > 0 {
		// For package-level functions with typed returns, use the return type
		// e.g., NewVue() *Vue should be grouped by Vue
		returnType := extractReturnType(decl.Type.Results.List[0].Type)
		if returnType != "" && ast.IsExported(returnType) {
			// Add the symbol to the list for constructors/factories
			*symbols = append(*symbols, AnalyzerSymbol{
				Package:  packageName,
				Filename: fileName,
				Symbol:   funcName,
				Receiver: returnType,
				Type:     "func",
				Default:  defaultFile,
				Pos:      decl.Pos(),
			})
		}
	}
}

// handleTypeDecl checks type declarations to ensure their names match expected filenames.
func handleTypeDecl(pass *analysis.Pass, decl *ast.GenDecl, fileName string, symbols *[]AnalyzerSymbol) {
	packageName := pass.Pkg.Name()

	for _, spec := range decl.Specs {
		// Ensure we are working with *ast.TypeSpec
		if t, ok := spec.(*ast.TypeSpec); ok {
			// Only consider structs, skip interfaces and other types
			if _, isStruct := t.Type.(*ast.StructType); !isStruct {
				continue
			}

			typeName := t.Name.Name

			if !ast.IsExported(typeName) {
				continue
			}

			// Base the default on the package name, e.g. service/service.go;
			defaultFile := packageName + "*.go"

			// Add the symbol to the list
			*symbols = append(*symbols, AnalyzerSymbol{
				Package:  packageName,
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

// getReceiverNameAndType extracts the receiver's name and determines if it's an interface.
func getReceiverNameAndType(pass *analysis.Pass, decl *ast.FuncDecl) (string, bool) {
	// If there is a receiver, return its name
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		recv := decl.Recv.List[0].Type
		var receiverName string

		if starExpr, ok := recv.(*ast.StarExpr); ok {
			// If receiver is a pointer, get its underlying type name
			if ident, ok := starExpr.X.(*ast.Ident); ok {
				receiverName = ident.Name
			}
		} else if ident, ok := recv.(*ast.Ident); ok {
			// If receiver is not a pointer, just return its name
			receiverName = ident.Name
		}

		if receiverName == "" {
			return "", false
		}

		// Check if the receiver type is an interface
		isInterface := isInterfaceType(pass, receiverName)
		return receiverName, isInterface
	}
	return "", false
}

// isInterfaceType checks if a given type name refers to an interface.
func isInterfaceType(pass *analysis.Pass, typeName string) bool {
	// Look up the type in the package scope
	obj := pass.Pkg.Scope().Lookup(typeName)
	if obj == nil {
		return false
	}

	// Check if the object is a type name
	typeObj, ok := obj.(*types.TypeName)
	if !ok {
		return false
	}

	// Check if the underlying type is an interface
	_, isInterface := typeObj.Type().(*types.Interface)
	return isInterface
}

// extractReturnType extracts the type name from a return type expression.
// Handles cases like *Vue, Vue, error, []string, etc.
func extractReturnType(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		// Direct type: Vue, error
		return e.Name
	case *ast.StarExpr:
		// Pointer type: *Vue
		return extractReturnType(e.X)
	case *ast.SelectorExpr:
		// Package-qualified type: foo.Vue
		return e.Sel.Name
	}
	return ""
}
