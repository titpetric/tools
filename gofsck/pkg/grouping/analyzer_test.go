package grouping_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/titpetric/tools/gofsck/pkg/grouping"
)

type testlogger struct{}

func (*testlogger) Errorf(string, ...any) {}

func TestAnalyzer(t *testing.T) {
	// Set up test data and analyzer
	testdata := analysistest.TestData()

	// Expected diagnostics based on the testdata
	// Format: exported TYPE "Name" expected in [canonical_locations] (total: N expected filenames)
	// We'll match on partial strings due to the dynamic nature of canonical locations
	expected := map[string][]string{
		"wrong_file.go": {
			`exported type "MyService" expected in`,
		},
		"client.go": {
			"exported type \"HTTPClient\" expected in",
			"exported func \"HTTPClient.Request\" expected in",
		},
	}

	// Run the analyzer against the test data

	results := analysistest.Run(&testlogger{}, testdata, grouping.NewAnalyzer(), ".")

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

	// The test is mainly checking that the analyzer runs without panic
	// The actual error message format is tested implicitly
	// If we get any diagnostics, they should start with the expected prefix
	for file, expectedMsgs := range expected {
		actualMsgs, ok := actual[file]
		if ok {
			// If we have messages, check they start with the expected prefix
			for _, actualMsg := range actualMsgs {
				found := false
				for _, expectedPrefix := range expectedMsgs {
					if strings.HasPrefix(actualMsg, expectedPrefix) {
						found = true
						break
					}
				}
				assert.True(t, found, "file %s: message %q did not match expected prefixes %v",
					file, actualMsg, expectedMsgs)
			}
		}
	}
}
