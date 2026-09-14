package docs

import (
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/model"
)

// TestPlantUMLSkipsAnonymousTypes covers the arrow that broke a diagram: a
// field declared with an anonymous struct spells its shape out, newlines and
// braces included, and plantuml stops on the line.
func TestPlantUMLSkipsAnonymousTypes(t *testing.T) {
	defs := model.DefinitionList{{
		Package: model.Package{ImportPath: "example.com/a", Package: "a"},
		Types: model.DeclarationList{{
			Kind: model.TypeKind,
			Name: "Config",
			Type: "struct",
			Fields: model.FieldList{
				{Name: "Admin", Type: "struct {\n\tTop []string `yaml:\"top\"`\n}"},
				{Name: "Name", Type: "string"},
			},
		}},
	}}

	var out strings.Builder
	if err := renderPlantUML(&out, Options{}, defs); err != nil {
		t.Fatal(err)
	}

	diagram := out.String()
	if strings.Contains(diagram, "struct {") {
		t.Errorf("an anonymous struct became an arrow target:\n%s", diagram)
	}
	if !strings.Contains(diagram, `class "a.Config"`) {
		t.Errorf("the declared type is missing:\n%s", diagram)
	}
}

// TestPlantUMLSkipsNamelessRelationships covers a type of a grouped block,
// whose name sits in Names: a foreign key arrow from a nameless source is a
// line plantuml stops on.
func TestPlantUMLSkipsNamelessRelationships(t *testing.T) {
	defs := model.DefinitionList{{
		Package: model.Package{ImportPath: "example.com/a", Package: "a"},
		Types: model.DeclarationList{{
			Kind:  model.TypeKind,
			Names: []string{"Session"},
			Type:  "struct",
			Fields: model.FieldList{
				{Name: "UserID", Type: "string"},
			},
		}},
	}}

	var out strings.Builder
	if err := renderPlantUML(&out, Options{}, defs); err != nil {
		t.Fatal(err)
	}

	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, `"a." `) {
			t.Errorf("an arrow from a nameless type: %s", line)
		}
	}
}

// TestImportsSkipsTestPackages covers the imports render over a parse that
// still holds the test packages: a _test package is not an edge of the tree.
func TestImportsSkipsTestPackages(t *testing.T) {
	imports := model.NewStringSet()
	imports.Add("a.go", `"example.com/a/inner"`)

	testImports := model.NewStringSet()
	testImports.Add("a_test.go", `"example.com/a/inner"`)

	defs := model.DefinitionList{
		{
			Package: model.Package{ImportPath: "example.com/a", Package: "a"},
			Imports: imports,
		},
		{
			Package: model.Package{ImportPath: "example.com/a_test", Package: "a_test", TestPackage: true},
			Imports: testImports,
		},
		{
			Package: model.Package{ImportPath: "example.com/a/inner", Package: "inner"},
		},
	}

	var out strings.Builder
	if err := renderImports(&out, defs); err != nil {
		t.Fatal(err)
	}

	diagram := out.String()
	if strings.Contains(diagram, "a_test") {
		t.Errorf("a test package became an edge:\n%s", diagram)
	}
	if !strings.Contains(diagram, "[example.com/a] --|> [example.com/a/inner]") {
		t.Errorf("the package edge is missing:\n%s", diagram)
	}
}
