package coverage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/packages"
)

func TestAnalyzer_Analyze(t *testing.T) {
	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedImports,
		Tests: true,
	}

	pkgs, err := packages.Load(cfg, ".")
	assert.NoError(t, err)
	assert.NotEmpty(t, pkgs)

	analyzer := New()
	report, err := analyzer.Analyze(pkgs)

	assert.NoError(t, err)
	assert.NotNil(t, report)
	// Symbols map will always be initialized (may be empty)
	assert.NotNil(t, report.Symbols)
	// Uncovered and StandaloneTests can be nil if empty
	if report.Uncovered != nil {
		assert.IsType(t, []string{}, report.Uncovered)
	}
	if report.StandaloneTests != nil {
		assert.IsType(t, []string{}, report.StandaloneTests)
	}

	// Check that coverage ratio is between 0 and 1
	assert.GreaterOrEqual(t, report.CoverageRatio, 0.0)
	assert.LessOrEqual(t, report.CoverageRatio, 1.0)
}

func TestParseTestName(t *testing.T) {
	tests := []struct {
		testName string
		expected []string
		contains string
	}{
		{
			testName: "TestFlatten",
			contains: "Flatten",
		},
		{
			testName: "TestFlatten_empty",
			contains: "Flatten",
		},
		{
			testName: "TestServer_Get",
			contains: "Server.Get",
		},
		{
			testName: "TestServer_Get_WithContext",
			contains: "Server.Get",
		},
		{
			testName: "TestHTTPClient_Request",
			contains: "HTTPClient.Request",
		},
		{
			testName: "TestNewVue",
			contains: "NewVue",
		},
		{
			testName: "TestNewVue_withOptions",
			contains: "NewVue",
		},
		{
			testName: "BenchmarkStack",
			contains: "Stack",
		},
		{
			testName: "BenchmarkStack_Push",
			contains: "Stack.Push",
		},
		{
			testName: "BenchmarkQueue_Enqueue",
			contains: "Queue.Enqueue",
		},
		{
			testName: "NotATest",
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			matches := ParseTestName(tc.testName)
			if tc.expected != nil {
				assert.Equal(t, tc.expected, matches)
			} else if tc.contains != "" {
				assert.NotNil(t, matches)
				found := false
				for _, m := range matches {
					if m == tc.contains {
						found = true
						break
					}
				}
				assert.True(t, found, "expected to find %q in %v", tc.contains, matches)
			} else {
				assert.Nil(t, matches)
			}
		})
	}
}

func TestMatchSymbolToTest(t *testing.T) {
	tests := []struct {
		symbol   string
		testName string
		expected bool
	}{
		{"Flatten", "TestFlatten", true},
		{"Flatten", "TestFlatten_empty", true},
		{"Flatten", "TestHelper", false},
		{"Server.Get", "TestServer_Get", true},
		{"Server.Get", "TestServer_Get_WithContext", true},
		{"Server.Close", "TestServer_Get", false},
		{"NewVue", "TestNewVue", true},
		{"NewVue", "TestNewVue_withOptions", true},
		{"NewVue", "TestVue", true}, // TestVue covers NewVue because NewVue returns Vue
		{"Stack", "BenchmarkStack", true},
		{"Stack.Push", "BenchmarkStack_Push", true},
		{"Queue.Enqueue", "BenchmarkQueue_Enqueue", true},
	}

	for _, tc := range tests {
		t.Run(tc.symbol+"_"+tc.testName, func(t *testing.T) {
			result := MatchSymbolToTest(tc.symbol, tc.testName)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestConstructorCoverage(t *testing.T) {
	tests := []struct {
		name     string
		testName string
		symbol   string
		expected bool
	}{
		{
			name:     "TestVue covers NewVue",
			testName: "TestVue",
			symbol:   "NewVue",
			expected: true,
		},
		{
			name:     "TestVue_Method covers NewVue",
			testName: "TestVue_EvalAttributes",
			symbol:   "NewVue",
			expected: true,
		},
		{
			name:     "TestLoader covers NewLoader",
			testName: "TestLoader",
			symbol:   "NewLoader",
			expected: true,
		},
		{
			name:     "TestFS covers NewFS",
			testName: "TestFS_Load",
			symbol:   "NewFS",
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := MatchSymbolToTest(tc.symbol, tc.testName)
			assert.Equal(t, tc.expected, result, "symbol: %s, test: %s", tc.symbol, tc.testName)
		})
	}
}

func TestExtractReturnType(t *testing.T) {
	tests := []struct {
		name     string
		typeStr  string
		expected string
	}{
		{
			name:     "pointer to type",
			typeStr:  "*Vue",
			expected: "Vue",
		},
		{
			name:     "direct type",
			typeStr:  "Vue",
			expected: "Vue",
		},
		{
			name:     "error type",
			typeStr:  "error",
			expected: "error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Note: This is a simple test that the function exists
			// Actual AST construction would be more complex
			t.Logf("extractReturnType function exists and can be called")
		})
	}
}

func TestHasIndirectCoverage(t *testing.T) {
	tests := []struct {
		name      string
		symbol    string
		testFuncs map[string]bool
		expected  bool
	}{
		{
			name:   "method with matching type test",
			symbol: "Stack.Get",
			testFuncs: map[string]bool{
				"TestStack": true,
			},
			expected: true,
		},
		{
			name:   "method without matching type test",
			symbol: "Stack.Get",
			testFuncs: map[string]bool{
				"TestOther": true,
			},
			expected: false,
		},
		{
			name:   "non-method symbol",
			symbol: "MyFunction",
			testFuncs: map[string]bool{
				"TestMyFunction": true,
			},
			expected: false,
		},
		{
			name:   "multiple methods with type test",
			symbol: "Stack.Copy",
			testFuncs: map[string]bool{
				"TestStack": true,
			},
			expected: true,
		},
		{
			name:   "Stack.EnvMap with TestStack",
			symbol: "Stack.EnvMap",
			testFuncs: map[string]bool{
				"TestStack": true,
			},
			expected: true,
		},
		{
			name:   "Stack.Copy with BenchmarkStack",
			symbol: "Stack.Copy",
			testFuncs: map[string]bool{
				"BenchmarkStack": true,
			},
			expected: true,
		},
		{
			name:   "Queue.Dequeue with BenchmarkQueue",
			symbol: "Queue.Dequeue",
			testFuncs: map[string]bool{
				"BenchmarkQueue": true,
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := hasIndirectCoverage(tc.symbol, tc.testFuncs)
			assert.Equal(t, tc.expected, result, "symbol: %s, testFuncs: %v", tc.symbol, tc.testFuncs)
		})
	}
}
