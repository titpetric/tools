package analyzer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/titpetric/tools/splint/analyzer"
)

func TestListPackages(t *testing.T) {
	targets, err := analyzer.ListPackages(".", "./...")
	assert.NoError(t, err)
	assert.NotEmpty(t, targets)

	// Every listed package carries the syntax it will be read from, and the
	// model entry the read fills in.
	for _, target := range targets {
		assert.NotNil(t, target.Syntax, target.Package.ImportPath)
		assert.NotEmpty(t, target.Package.ImportPath)
	}

	paths := targets.Packages()
	assert.Len(t, paths, len(targets))
}
