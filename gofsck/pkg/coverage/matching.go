package coverage

import (
	"fmt"
	"strings"
)

// ParseTestName extracts the symbol it tests from a test or benchmark function name.
// Supports patterns like:
// - TestSymbol / BenchmarkSymbol
// - TestSymbol_context / BenchmarkSymbol_context
// - TestReceiver_Method / BenchmarkReceiver_Method
// - TestReceiver_Method_context / BenchmarkReceiver_Method_context
func ParseTestName(testName string) []string {
	// Remove Test or Benchmark prefix
	var name string
	if strings.HasPrefix(testName, "Test") {
		name = strings.TrimPrefix(testName, "Test")
	} else if strings.HasPrefix(testName, "Benchmark") {
		name = strings.TrimPrefix(testName, "Benchmark")
	} else {
		return nil
	}

	if name == "" {
		return nil
	}

	// Split by underscore to handle receiver_method pattern
	parts := strings.Split(name, "_")
	if len(parts) == 0 {
		return nil
	}

	// Try different combinations for receiver_method pattern
	var matches []string

	// Single symbol: TestSymbol or TestSymbol_context
	matches = append(matches, parts[0])

	// Receiver.Method pattern: TestReceiver_Method or TestReceiver_Method_context
	if len(parts) >= 2 {
		// First two parts might be Receiver_Method
		receiverMethod := fmt.Sprintf("%s.%s", parts[0], parts[1])
		matches = append(matches, receiverMethod)
	}

	return matches
}

// MatchSymbolToTest returns true if a test name matches a symbol.
// symbolName can be either "FunctionName" or "ReceiverType.MethodName"
// Also handles constructor matching: TestVue matches NewVue because NewVue returns Vue
func MatchSymbolToTest(symbolName, testName string) bool {
	possibleMatches := ParseTestName(testName)
	for _, match := range possibleMatches {
		if match == symbolName {
			return true
		}

		// For constructors: if symbolName is "NewX" and match is "X", it's a match
		// This aligns with grouping analyzer which groups NewVue into vue.go
		if !strings.Contains(match, ".") && strings.HasPrefix(symbolName, "New") {
			expectedConstructor := "New" + match
			if expectedConstructor == symbolName {
				return true
			}
		}
	}
	return false
}
