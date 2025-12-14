package pairing

import (
	"strings"

	"golang.org/x/tools/go/packages"
)

// Analyzer performs file-test pairing analysis on a set of packages.
type Analyzer struct{}

// New creates a new file-test pairing analyzer.
func New() *Analyzer {
	return &Analyzer{}
}

// Analyze examines packages and returns pairing statistics.
func (a *Analyzer) Analyze(pkgs []*packages.Package) (*Report, error) {
	files := make(map[string]bool)
	tests := make(map[string]bool)
	paired := 0

	// First pass: collect all Go files (GoFiles includes both regular and test files)
	for _, pkg := range pkgs {
		for _, file := range pkg.GoFiles {
			baseName := extractBaseName(file)
			if isTestFile(file) {
				tests[baseName] = true
			} else {
				files[baseName] = true
			}
		}
	}

	// Count paired files
	for baseName := range files {
		if tests[baseName] {
			paired++
		}
	}

	standaloneFiles := len(files) - paired
	standaloneTests := len(tests) - paired

	return &Report{
		Files:           len(files),
		Tests:           len(tests),
		Paired:          paired,
		StandaloneFiles: standaloneFiles,
		StandaloneTests: standaloneTests,
	}, nil
}

// isTestFile returns true if the filename ends with _test.go
func isTestFile(filename string) bool {
	return strings.HasSuffix(filename, "_test.go")
}

// extractBaseName removes the _test.go or .go suffix from a filename
func extractBaseName(filename string) string {
	if isTestFile(filename) {
		return strings.TrimSuffix(filename, "_test.go")
	}
	return strings.TrimSuffix(filename, ".go")
}
