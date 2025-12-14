package pairing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/packages"
)

func TestAnalyzer_Analyze(t *testing.T) {
	// Load test packages with Tests flag to include test files
	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedFiles,
		Tests: true,
	}

	pkgs, err := packages.Load(cfg, ".")
	assert.NoError(t, err)
	assert.NotEmpty(t, pkgs)

	analyzer := New()
	report, err := analyzer.Analyze(pkgs)

	assert.NoError(t, err)
	assert.NotNil(t, report)

	// The pairing package itself has analyzer.go and analyzer_test.go
	assert.Greater(t, report.Files, 0)
	assert.Greater(t, report.Tests, 0)
	assert.GreaterOrEqual(t, report.Paired, 0)
}

func TestExtractBaseName(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"service.go", "service"},
		{"service_test.go", "service"},
		{"http_client.go", "http_client"},
		{"http_client_test.go", "http_client"},
		{"main.go", "main"},
		{"main_test.go", "main"},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			got := extractBaseName(tc.filename)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"service_test.go", true},
		{"service.go", false},
		{"main_test.go", true},
		{"main.go", false},
		{"test.go", false},
		{"_test.go", true},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			got := isTestFile(tc.filename)
			assert.Equal(t, tc.expected, got)
		})
	}
}
