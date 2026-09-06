package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/titpetric/tools/splint/model"
)

// TestBaseName covers the name a path is reached by, which is what decides
// whether an import is used and what an alias would have to say.
func TestBaseName(t *testing.T) {
	tests := []struct{ path, name string }{
		{"context", "context"},
		{"net/http", "http"},
		{"github.com/lib/pq", "pq"},
		{"example.com/x/v2", "x"},
		{"charm.land/lipgloss/v2", "lipgloss"},
		{"gopkg.in/yaml.v3", "yaml"},
		{"gopkg.in/check.v1", "check"},
		{"github.com/goccy/go-yaml", "go-yaml"},
		{"example.com/v2", "example.com"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			assert.Equal(t, test.name, model.BaseName(test.path))
		})
	}
}

// TestIsPackageName covers which of those names Go would accept written as an
// alias. The two that are not are why a path does not always imply a name.
func TestIsPackageName(t *testing.T) {
	valid := []string{"context", "http", "pq", "x", "lipgloss", "yaml", "_x", "utf8", "\u00fcber"}
	invalid := []string{"", "go-yaml", "example.com", "2fast", "a.b", "a/b", "a b"}

	for _, name := range valid {
		assert.True(t, model.IsPackageName(name), "%q should be a package name", name)
	}
	for _, name := range invalid {
		assert.False(t, model.IsPackageName(name), "%q should not be a package name", name)
	}
}

// TestCgoAndFree cover the one declaration a formatter may not move.
func TestCgoAndFree(t *testing.T) {
	decls := model.ImportDeclList{
		{Line: 3, EndLine: 3, Specs: []model.ImportSpec{{Path: "C"}}},
		{Line: 5, EndLine: 8, Block: true, Specs: []model.ImportSpec{{Path: "fmt"}, {Path: "unsafe"}}},
	}

	assert.True(t, decls[0].Cgo())
	assert.False(t, decls[1].Cgo())

	free := decls.Free()
	assert.Len(t, free, 1)
	assert.Equal(t, 5, free[0].Line, "the block beside the import of C is the one a rewrite may write")
}
