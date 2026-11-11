package analyzer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/titpetric/tools/splint/parsers/analyzer"
)

func TestListPackages(t *testing.T) {
	targets, err := analyzer.ListPackages(".", "./...")
	assert.NoError(t, err)
	assert.NotEmpty(t, targets)

	// Every listed package holds the syntax the read takes. The
	// model entry the read fills in.
	for _, target := range targets {
		assert.NotNil(t, target.Syntax, target.Package.ImportPath)
		assert.NotEmpty(t, target.Package.ImportPath)
	}

	paths := targets.Packages()
	assert.Len(t, paths, len(targets))
}
