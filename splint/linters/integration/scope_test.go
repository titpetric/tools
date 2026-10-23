package integration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/titpetric/tools/splint/model"
)

// TestScope follows the helpers of the test files: through a method of a
// test type, past two helpers calling each other, and down to the literal,
// and stops at a chain longer than the linter's depth.
func TestScope(t *testing.T) {
	t.Parallel()

	fn := func(name, receiver string, globals model.StringSet, literals ...string) *model.Declaration {
		return &model.Declaration{
			Kind: model.FuncKind, Name: name, Receiver: receiver, File: "x_test.go", Line: 1,
			Globals: globals, Literals: literals,
		}
	}
	root := &model.DocumentRoot{Packages: model.DefinitionList{{
		Package: model.Package{Package: "x", Path: "./x", TestPackage: true},
		Files:   model.FileList{{Name: "x_test.go", Test: true}},
		Funcs: model.DeclarationList{
			fn("TestHarness", "", model.StringSet{"harness": {"start"}}),
			fn("start", "*harness", model.StringSet{"open": nil}),
			fn("open", "", nil, "file:x?mode=memory"),
			fn("TestLoop", "", model.StringSet{"ping": nil}),
			fn("ping", "", model.StringSet{"pong": nil}),
			fn("pong", "", model.StringSet{"ping": nil}),
		},
	}}}

	_, issues := lint(t, enabled(), root)

	assert.Len(t, issues, 1, "two helpers calling each other cross nothing")
	reason, _ := issues[0].Attr("reason")
	assert.Equal(t, "TestHarness", issues[0].Symbol)
	assert.Equal(t, `calls x.harness.start → x.open, which opens sqlite "file:x?mode=memory"`, reason)

	shallow := enabled()
	shallow.Depth = 1
	_, issues = lint(t, shallow, root)
	assert.Empty(t, issues, "a chain two calls deep is past a depth of one")
}
