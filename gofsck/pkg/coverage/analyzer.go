package coverage

import (
	"fmt"
	"go/ast"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Analyzer performs symbol-test coverage analysis.
type Analyzer struct{}

// New creates a new symbol-test coverage analyzer.
func New() *Analyzer {
	return &Analyzer{}
}

// Analyze examines packages and returns symbol-test coverage information.
func (a *Analyzer) Analyze(pkgs []*packages.Package) (*Report, error) {
	symbols := make(map[string]bool)           // exported symbols
	testFuncs := make(map[string]bool)         // test function names
	symbolToTests := make(map[string][]string) // symbol -> tests that cover it
	testFileSymbols := make(map[string]bool)   // symbols defined in test files (shouldn't be in uncovered list)

	// Iterate through packages
	for _, pkg := range pkgs {
		// Skip test packages
		if strings.HasSuffix(pkg.PkgPath, ".test") {
			continue
		}

		// Process each file in the package
		if pkg.Syntax != nil {
			for _, file := range pkg.Syntax {
				a.analyzeFile(file, pkg, symbolToTests, symbols, testFuncs, testFileSymbols)
			}
		}
	}

	// Find uncovered symbols (excluding those defined in test files)
	var uncovered []string
	for symbol := range symbols {
		// Skip symbols defined in test files (they're test helpers, not part of the public API)
		if testFileSymbols[symbol] {
			continue
		}

		if _, hasCoverage := symbolToTests[symbol]; !hasCoverage {
			// Check if this is a method with indirect coverage via a type test
			if !hasIndirectCoverage(symbol, testFuncs) {
				uncovered = append(uncovered, symbol)
			}
		}
	}

	// Find standalone tests (tests with no matching symbol that exists)
	var standaloneTests []string
	for testName := range testFuncs {
		found := false
		// Check if this test matches any symbol that actually exists
		for symbol, tests := range symbolToTests {
			for _, t := range tests {
				if t == testName {
					// Found the test, but only count it as non-standalone if the symbol exists
					if _, symbolExists := symbols[symbol]; symbolExists {
						found = true
						break
					}
				}
			}
			if found {
				break
			}
		}
		if !found {
			standaloneTests = append(standaloneTests, testName)
		}
	}

	// Sort uncovered and standalone tests for consistent output
	sort.Strings(uncovered)
	sort.Strings(standaloneTests)

	// Calculate coverage ratio
	totalSymbols := len(symbols)
	coveredSymbols := len(symbols) - len(uncovered)
	coverageRatio := 0.0
	if totalSymbols > 0 {
		coverageRatio = float64(coveredSymbols) / float64(totalSymbols)
	}

	return &Report{
		Symbols:         symbolToTests,
		Uncovered:       uncovered,
		StandaloneTests: standaloneTests,
		CoverageRatio:   coverageRatio,
	}, nil
}

// analyzeFile processes a single AST file to extract symbols and tests.
func (a *Analyzer) analyzeFile(file *ast.File, pkg *packages.Package,
	symbolToTests map[string][]string, symbols map[string]bool, testFuncs map[string]bool,
	testFileSymbols map[string]bool) {

	// Check if this is a test file
	isTestFile := strings.HasSuffix(file.Name.Name, "_test.go")

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			funcName := d.Name.Name

			// Check if this is a test or benchmark function (before treating as exported symbol)
			if (strings.HasPrefix(funcName, "Test") || strings.HasPrefix(funcName, "Benchmark")) && d.Recv == nil {
				testFuncs[funcName] = true
				// Try to match this test to symbols
				a.matchTestToSymbols(funcName, symbolToTests)
				// Don't process as a symbol
				continue
			}

			// Check if this is an exported function
			if ast.IsExported(funcName) {
				receiver := getReceiverName(d)
				if receiver != "" {
					// Method on exported receiver
					if ast.IsExported(receiver) {
						symbolName := fmt.Sprintf("%s.%s", receiver, funcName)
						symbols[symbolName] = true
						if isTestFile {
							testFileSymbols[symbolName] = true
						}
					}
				} else {
					// Package-level function
					symbols[funcName] = true
					if isTestFile {
						testFileSymbols[funcName] = true
					}

					// For constructors like NewVue() *Vue, also track the return type
					// This allows TestVue to cover NewVue
					// Only add if it's a struct type (not interface or other types)
					if strings.HasPrefix(funcName, "New") && d.Type.Results != nil && len(d.Type.Results.List) > 0 {
						returnExpr := d.Type.Results.List[0].Type
						// Check if the return type is a struct before adding it
						if isStructType(pkg, returnExpr) {
							returnType := extractReturnType(returnExpr)
							if returnType != "" && ast.IsExported(returnType) {
								// Also add as if it were a type symbol for test matching
								symbols[returnType] = true
								if isTestFile {
									testFileSymbols[returnType] = true
								}
							}
						}
					}
				}
			}

		case *ast.GenDecl:
			// Check for type declarations
			if d.Tok.String() == "type" {
				for _, spec := range d.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						// Only track exported struct types
						if ast.IsExported(ts.Name.Name) {
							if _, isStruct := ts.Type.(*ast.StructType); isStruct {
								symbols[ts.Name.Name] = true
								if isTestFile {
									testFileSymbols[ts.Name.Name] = true
								}
							}
						}
					}
				}
			}
		}
	}
}

// matchTestToSymbols matches a test function to all possible symbols it could test.
func (a *Analyzer) matchTestToSymbols(testName string, symbolToTests map[string][]string) {
	possibleMatches := ParseTestName(testName)
	for _, match := range possibleMatches {
		symbolToTests[match] = append(symbolToTests[match], testName)

		// For type names, also match constructor functions (e.g., TestVue matches NewVue)
		// This aligns with grouping analyzer which groups NewVue into vue.go
		if !strings.Contains(match, ".") && match != "" {
			constructorName := "New" + match
			symbolToTests[constructorName] = append(symbolToTests[constructorName], testName)
		}
	}
}

// hasIndirectCoverage checks if a symbol (like "Type.Method") has indirect coverage.
// If a TestType or BenchmarkType exists, all methods on Type are considered indirectly covered.
func hasIndirectCoverage(symbol string, testFuncs map[string]bool) bool {
	// Check if this is a method symbol (contains a dot)
	if !strings.Contains(symbol, ".") {
		return false
	}

	// Extract the receiver type (e.g., "Stack" from "Stack.Get")
	parts := strings.Split(symbol, ".")
	if len(parts) != 2 {
		return false
	}

	receiverType := parts[0]

	// Check if TestReceiverType or BenchmarkReceiverType exists
	testName := fmt.Sprintf("Test%s", receiverType)
	benchmarkName := fmt.Sprintf("Benchmark%s", receiverType)
	return testFuncs[testName] || testFuncs[benchmarkName]
}

// isStructType checks if an expression represents a struct type.
func isStructType(pkg *packages.Package, expr ast.Expr) bool {
	// Unwrap pointer types
	if starExpr, ok := expr.(*ast.StarExpr); ok {
		expr = starExpr.X
	}

	// Get the type name
	var typeName string
	switch e := expr.(type) {
	case *ast.Ident:
		typeName = e.Name
	case *ast.SelectorExpr:
		typeName = e.Sel.Name
	default:
		return false
	}

	if typeName == "" {
		return false
	}

	// Look up the type in the package types
	if pkg.Types != nil {
		obj := pkg.Types.Scope().Lookup(typeName)
		if obj != nil {
			if typeObj, ok := obj.(*types.TypeName); ok {
				// Check if it's a struct type
				_, isStruct := typeObj.Type().Underlying().(*types.Struct)
				return isStruct
			}
		}
	}

	return false
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
