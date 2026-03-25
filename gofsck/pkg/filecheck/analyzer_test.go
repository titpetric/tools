package filecheck

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/packages"
)

func TestAnalyzer_Analyze(t *testing.T) {
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

	// The filecheck package itself has .go files
	var goGroup *ScannedGroup
	for i := range report.Scanned {
		if report.Scanned[i].Ext == ".go" {
			goGroup = &report.Scanned[i]
			break
		}
	}
	assert.NotNil(t, goGroup)
	assert.Greater(t, goGroup.Files, 0)
	assert.Len(t, goGroup.Histogram, numBuckets)

	// Rating should be between 0 and 100
	assert.GreaterOrEqual(t, report.Rating, 0.0)
	assert.LessOrEqual(t, report.Rating, 100.0)
}

func TestFileBucket(t *testing.T) {
	tests := []struct {
		name     string
		size     int64
		expected int
	}{
		{"tiny file", 500, 0},
		{"1KB boundary", 1024, 0},
		{"just over 1KB", 1025, 1},
		{"2KB boundary", 2048, 1},
		{"4KB file", 4096, 2},
		{"8KB file", 8192, 3},
		{"16KB file", 16384, 4},
		{"32KB file", 32768, 5},
		{"64KB file", 65536, 6},
		{"large file", 100000, 7},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := fileBucket(tc.size)
			assert.Equal(t, tc.expected, got)
		})
	}
}
