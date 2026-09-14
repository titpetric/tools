package docs

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/titpetric/tools/splint/model"
)

// configDefs is a small config tree: a root reaching a nested type through a
// pointer and a slice, an unreferenced type, and a typedef over a declared
// one.
func configDefs() model.DefinitionList {
	return model.DefinitionList{{
		Package: model.Package{ImportPath: "example.com/config", Package: "config"},
		Types: model.DeclarationList{
			{
				Kind: model.TypeKind,
				Name: "Aside",
				Doc:  "Aside is declared first and reached by nothing.",
				Fields: model.FieldList{
					{Name: "Note", Type: "string", Doc: "Note says something."},
				},
			},
			{
				Kind: model.TypeKind,
				Name: "Config",
				Doc:  "Config is the root.",
				Fields: model.FieldList{
					{Name: "Server", Type: "*Server", Doc: "Server configures the service."},
					{Name: "Endpoints", Type: "[]*Endpoint", JSONName: "endpoints", Doc: "Endpoints lists the endpoints."},
					{Name: "Debug", Type: "bool", Doc: "Debug turns the logging up."},
					{Name: "Extra", Type: "map[string]interface{}"},
				},
			},
			{
				Kind: model.TypeKind,
				Name: "Endpoint",
				Fields: model.FieldList{
					{Name: "Path", Type: "string", Doc: "Path is the route."},
				},
			},
			{
				Kind: model.TypeKind,
				Name: "Endpoints",
				Type: "[]*Endpoint",
				Doc:  "Endpoints is a list of Endpoint.",
			},
			{
				Kind: model.TypeKind,
				Name: "Server",
				Fields: model.FieldList{
					{Name: "Addr", Type: "string", Doc: "Addr is listened on."},
				},
			},
		},
	}}
}

// TestRenderConfig covers the reference: the root first under the title, the
// reached types after it in reach order, links on declared types, and the
// unreachable type at the end.
func TestRenderConfig(t *testing.T) {
	var out strings.Builder
	err := renderConfig(&out, Options{Root: "Config", Title: "# Configuration"}, configDefs())
	require.NoError(t, err)

	page := out.String()

	require.Contains(t, page, "# Configuration\n\nConfig is the root.")
	require.NotContains(t, page, "# Config\n", "the title replaces the root heading")

	require.Contains(t, page, "**Field: `Server` ([Server](#server))**\nServer configures the service.")
	require.Contains(t, page, "**Field: `endpoints` ([[]*Endpoint](#endpoint))**", "the json name and the raw slice type")
	require.Contains(t, page, "**Field: `Debug` (`boolean`)**")
	require.Contains(t, page, "**Field: `Extra` (`any`)**")

	rootAt := strings.Index(page, "# Configuration")
	serverAt := strings.Index(page, "# Server")
	asideAt := strings.Index(page, "# Aside")
	require.True(t, rootAt < serverAt, "a reached type follows the root")
	require.True(t, serverAt < asideAt, "an unreached type comes last")
}

// TestRenderConfigTypedef covers a type with no fields of its own, which is
// described by what it is defined as.
func TestRenderConfigTypedef(t *testing.T) {
	var out strings.Builder
	err := renderConfig(&out, Options{Root: "Config"}, configDefs())
	require.NoError(t, err)

	require.Contains(t, out.String(), "Type defined as array of `Endpoint` values, see [Endpoint](#endpoint) definition.")
	require.Contains(t, out.String(), "# Config\n", "without a title the root keeps its heading")
}

// TestRenderConfigUnknownRoot covers the refusal: a root nothing declares is
// a reference of nothing.
func TestRenderConfigUnknownRoot(t *testing.T) {
	var out strings.Builder
	err := renderConfig(&out, Options{Root: "Nowhere"}, configDefs())
	require.Error(t, err)
}
