package gofsck_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/titpetric/tools/gofsck/pkg/gofsck" // Correct import path with /pkg/gofsck
)

type testlogger struct{}

func (*testlogger) Errorf(string, ...any) {}

func TestAnalyzer(t *testing.T) {
	// Set up test data and analyzer
	testdata := analysistest.TestData()

	// Expected diagnostics based on the testdata (formatted as "file.go: exported type %q does not match filename")
	expected := map[string][]string{
		"wrong_file.go": {
			`exported type "MyService" does not match filename or fallback to types.go`,
		},
	}

	// Run the analyzer against the test data

	results := analysistest.Run(&testlogger{}, testdata, gofsck.NewAnalyzer(), ".")

	// Map to hold diagnostics grouped by file name and message
	actual := map[string][]string{}

	// Iterate over results and capture diagnostics
	for _, result := range results {
		// Each result may contain diagnostics
		for _, diag := range result.Diagnostics {
			// Only add diagnostics where the message is not empty
			if diag.Message != "" {
				// Split the message to extract the filename (before ": ")
				parts := strings.SplitN(diag.Message, ": ", 2)
				if len(parts) == 2 {
					filename := func() string {
						name := strings.SplitN(parts[0], ":", 2)
						return filepath.Base(name[0])
					}()
					actual[filename] = append(actual[filename], parts[1])
				}
			}
		}
	}

	assert.Equal(t, expected, actual)
}
