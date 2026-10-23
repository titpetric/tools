package testfacts_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/parsers/testfacts"
)

func TestFill(t *testing.T) {
	t.Parallel()

	test := &model.Declaration{
		Kind: model.FuncKind,
		Name: "TestOpen",
		File: "store_test.go",
		Source: "// TestOpen opens a store.\nfunc TestOpen(t *testing.T) {\n" +
			"\tdb := open(\":memory:\") // \"not a literal\"\n" +
			"\trun(`sh`, \"-c\", \"true\", `sh`)\n" +
			"\tquery := `SELECT 1\nFROM x`\n" +
			"\t_ = 'r'\n}",
	}
	code := &model.Declaration{
		Kind:   model.FuncKind,
		Name:   "Open",
		File:   "store.go",
		Source: "func Open() { open(\"file.db\") }",
	}
	def := &model.Definition{Funcs: model.DeclarationList{test, code}}

	testfacts.Fill(def)

	assert.Equal(t, []string{"-c", ":memory:", "SELECT 1\nFROM x", "sh", "true"}, test.Literals)
	assert.Empty(t, code.Literals, "a file outside the tests carries no literals")
}
