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

// symbolIndex collects symbol and test data during analysis.
type symbolIndex struct {
	symbols         map[string]bool     // exported symbols
	symbolPkgPath   map[string]string   // symbol -> package path
	constructors    map[string]bool     // constructor functions (New*)
	testFuncs       map[string]bool     // test function names
	symbolToTests   map[string][]string // symbol -> tests that cover it
	testFileSymbols map[string]bool     // symbols defined in test files
}

func newSymbolIndex() *symbolIndex {
	return &symbolIndex{
		symbols:         make(map[string]bool),
		symbolPkgPath:   make(map[string]string),
		constructors:    make(map[string]bool),
		testFuncs:       make(map[string]bool),
		symbolToTests:   make(map[string][]string),
		testFileSymbols: make(map[string]bool),
	}
}

// integrationPackageSuffixes identifies packages whose symbols require integration tests.
var integrationPackageSuffixes = []string{
	"/storage",
	"/repository",
}

// testKind returns "integration" or "unit" based on the symbol's package path.
func testKind(pkgPath string) string {
	for _, suffix := range integrationPackageSuffixes {
		if strings.HasSuffix(pkgPath, suffix) {
			return "integration"
		}
	}
	return "unit"
}

// matchTestToSymbols matches a test function to all possible symbols it could test.
func (idx *symbolIndex) matchTestToSymbols(testName string) {
	possibleMatches := ParseTestName(testName)
	for _, match := range possibleMatches {
		idx.symbolToTests[match] = append(idx.symbolToTests[match], testName)

		// For type names, also match constructor functions (e.g., TestVue matches NewVue)
		if !strings.Contains(match, ".") && match != "" {
			constructorName := "New" + match
			idx.symbolToTests[constructorName] = append(idx.symbolToTests[constructorName], testName)
		}
	}
}

// resolveReceiverCoverage maps receiver-level tests (TestServer) to all
// method symbols on that receiver (Server.Get, Server.Close, etc.).
func (idx *symbolIndex) resolveReceiverCoverage() {
	for symbol := range idx.symbols {
		if _, hasCoverage := idx.symbolToTests[symbol]; hasCoverage {
			continue
		}

		parts := strings.SplitN(symbol, ".", 2)
		if len(parts) != 2 {
			continue
		}

		receiver := parts[0]
		testName := "Test" + receiver
		benchName := "Benchmark" + receiver

		if idx.testFuncs[testName] {
			idx.symbolToTests[symbol] = append(idx.symbolToTests[symbol], testName)
		}
		if idx.testFuncs[benchName] {
			idx.symbolToTests[symbol] = append(idx.symbolToTests[symbol], benchName)
		}
	}
}

// Analyze examines packages and returns symbol-test coverage information.
func (a *Analyzer) Analyze(pkgs []*packages.Package) (*Report, error) {
	idx := newSymbolIndex()

	// Iterate through packages
	for _, pkg := range pkgs {
		// Skip test packages
		if strings.HasSuffix(pkg.PkgPath, ".test") {
			continue
		}

		// Process each file in the package
		if pkg.Syntax != nil {
			for _, file := range pkg.Syntax {
				a.analyzeFile(file, pkg, idx)
			}
		}
	}

	// Resolve receiver-level test coverage for methods
	idx.resolveReceiverCoverage()

	// Find uncovered symbols (excluding constructors and those defined in test files)
	var uncovered []UncoveredSymbol
	for symbol := range idx.symbols {
		// Skip symbols defined in test files (they're test helpers, not part of the public API)
		if idx.testFileSymbols[symbol] {
			continue
		}

		// Skip constructors — they are covered by their return type's tests
		if idx.constructors[symbol] {
			continue
		}

		// Skip methods — only require tests for receiver types, not individual methods
		if strings.Contains(symbol, ".") {
			continue
		}

		if _, hasCoverage := idx.symbolToTests[symbol]; !hasCoverage {
			uncovered = append(uncovered, UncoveredSymbol{
				Symbol:       symbol,
				ExpectedTest: ExpectedTestName(symbol),
				TestKind:     testKind(idx.symbolPkgPath[symbol]),
			})
		}
	}

	// Find standalone tests (tests with no matching symbol that exists)
	var standaloneTests []string
	for testName := range idx.testFuncs {
		found := false
		// Check if this test matches any symbol that actually exists
		for symbol, tests := range idx.symbolToTests {
			for _, t := range tests {
				if t == testName {
					// Found the test, but only count it as non-standalone if the symbol exists
					if _, symbolExists := idx.symbols[symbol]; symbolExists {
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
	sort.Slice(uncovered, func(i, j int) bool {
		return uncovered[i].Symbol < uncovered[j].Symbol
	})
	sort.Strings(standaloneTests)

	// Calculate coverage ratio based on eligible symbols only
	// (excludes constructors and test-file symbols from the total)
	eligible := 0
	for symbol := range idx.symbols {
		if idx.testFileSymbols[symbol] || idx.constructors[symbol] || strings.Contains(symbol, ".") {
			continue
		}
		eligible++
	}
	covered := eligible - len(uncovered)
	constructorCount := len(idx.constructors)
	coverageRatio := 0.0
	if eligible > 0 {
		coverageRatio = float64(covered) / float64(eligible)
	}
	adjustedCoverage := 0.0
	adjustedTotal := eligible + constructorCount
	if adjustedTotal > 0 {
		adjustedCoverage = float64(covered+constructorCount) / float64(adjustedTotal)
	}

	wantUnit, wantIntegration := 0, 0
	for _, u := range uncovered {
		if u.TestKind == "integration" {
			wantIntegration++
		} else {
			wantUnit++
		}
	}

	return &Report{
		Symbols:          idx.symbolToTests,
		Covered:          covered,
		Uncovered:        uncovered,
		Constructors:     constructorCount,
		StandaloneTests:  standaloneTests,
		CoverageRatio:    coverageRatio,
		AdjustedCoverage: adjustedCoverage,
		WantUnit:         wantUnit,
		WantIntegration:  wantIntegration,
	}, nil
}

// analyzeFile processes a single AST file to extract symbols and tests.
func (a *Analyzer) analyzeFile(file *ast.File, pkg *packages.Package, idx *symbolIndex) {
	// Check if this is a test file
	isTestFile := strings.HasSuffix(file.Name.Name, "_test.go")

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			funcName := d.Name.Name

			// Check if this is a test or benchmark function (before treating as exported symbol)
			if (strings.HasPrefix(funcName, "Test") || strings.HasPrefix(funcName, "Benchmark")) && d.Recv == nil {
				idx.testFuncs[funcName] = true
				idx.matchTestToSymbols(funcName)
				continue
			}

			// Check if this is an exported function
			if ast.IsExported(funcName) {
				receiver := getReceiverName(d)
				if receiver != "" {
					// Method on exported receiver
					if ast.IsExported(receiver) {
						symbolName := fmt.Sprintf("%s.%s", receiver, funcName)
						idx.symbols[symbolName] = true
						idx.symbolPkgPath[symbolName] = pkg.PkgPath
						if isTestFile {
							idx.testFileSymbols[symbolName] = true
						}
					}
				} else {
					// Package-level function
					idx.symbols[funcName] = true
					idx.symbolPkgPath[funcName] = pkg.PkgPath
					if isTestFile {
						idx.testFileSymbols[funcName] = true
					}

					// Track constructors (functions starting with New, no receiver)
					if strings.HasPrefix(funcName, "New") {
						idx.constructors[funcName] = true
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
								idx.symbols[returnType] = true
								idx.symbolPkgPath[returnType] = pkg.PkgPath
								if isTestFile {
									idx.testFileSymbols[returnType] = true
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
								idx.symbols[ts.Name.Name] = true
								idx.symbolPkgPath[ts.Name.Name] = pkg.PkgPath
								if isTestFile {
									idx.testFileSymbols[ts.Name.Name] = true
								}
							}
						}
					}
				}
			}
		}
	}
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
