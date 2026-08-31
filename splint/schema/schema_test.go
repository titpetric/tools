package schema_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/schema"
)

// document is a tree holding two packages, one of which declares a type the
// other declares under the same name.
func document() *model.DocumentRoot {
	return &model.DocumentRoot{Packages: model.DefinitionList{
		{
			Package: model.Package{Package: "client", ImportPath: "example.com/client", Path: "./client"},
			Types: model.DeclarationList{
				{
					Kind: model.TypeKind, Name: "Config", Doc: "Config is what a client is built from.",
					Fields: model.FieldList{
						{Name: "Addr", Type: "string", JSONName: "addr"},
						{Name: "Retries", Type: "int", JSONName: "retries"},
						{Name: "Verbose", Type: "bool", JSONName: "verbose"},
					},
				},
				// An interface has no shape a schema can hold.
				{Kind: model.TypeKind, Name: "Store", Type: "interface"},
			},
		},
		{
			Package: model.Package{Package: "server", ImportPath: "example.com/server", Path: "./server"},
			Types: model.DeclarationList{{
				Kind: model.TypeKind, Name: "Config",
				Fields: model.FieldList{{Name: "Listen", Type: "string", JSONName: "listen"}},
			}},
		},
		// A test package is not a shape anyone encodes.
		{
			Package: model.Package{Package: "client_test", Path: "./client", TestPackage: true},
			Types:   model.DeclarationList{{Kind: model.TypeKind, Name: "Fixture", Fields: model.FieldList{{Name: "A", Type: "string", JSONName: "a"}}}},
		},
	}}
}

func TestConvert(t *testing.T) {
	got, err := schema.Convert(document(), schema.Options{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	if got.Schema != "http://json-schema.org/draft-07/schema#" {
		t.Errorf("$schema = %q", got.Schema)
	}

	config, ok := got.Definitions["Config"]
	if !ok {
		t.Fatalf("definitions = %v, want Config", keys(got.Definitions))
	}
	if config.Description != "Config is what a client is built from." {
		t.Errorf("Config description = %q", config.Description)
	}
	for _, field := range []string{"addr", "retries", "verbose"} {
		if _, ok := config.Properties[field]; !ok {
			t.Errorf("Config has no %q property: %v", field, keys(config.Properties))
		}
	}
	if got := config.Properties["retries"].Type; got != "integer" {
		t.Errorf("retries is %q, want integer", got)
	}
	if got := config.Properties["verbose"].Type; got != "boolean" {
		t.Errorf("verbose is %q, want boolean", got)
	}

	// A name two packages both declare is qualified with the package: a schema
	// has one namespace and two Config types are not the same thing.
	if _, ok := got.Definitions["server.Config"]; !ok {
		t.Errorf("definitions = %v, want the second Config qualified", keys(got.Definitions))
	}

	// An interface describes no shape, and a test package is not a shape
	// anyone encodes.
	if _, ok := got.Definitions["Store"]; ok {
		t.Error("definitions hold an interface")
	}
	if _, ok := got.Definitions["Fixture"]; ok {
		t.Error("definitions hold a type from a test package")
	}
}

func TestWrite(t *testing.T) {
	var out bytes.Buffer
	if err := schema.Write(&out, document(), schema.Options{}); err != nil {
		t.Fatal(err)
	}

	// The output is a JSON Schema document and nothing else: a library that
	// logged what it skipped would put it in the caller's output.
	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("Write() did not write a JSON document: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "\n    ") {
		t.Error("Write() did not indent the document")
	}
}

func TestWriteOnAnEmptyDocument(t *testing.T) {
	var out bytes.Buffer
	if err := schema.Write(&out, &model.DocumentRoot{}, schema.Options{}); err != nil {
		t.Fatal(err)
	}

	var decoded struct {
		Definitions map[string]any `json:"definitions"`
	}
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Definitions) != 0 {
		t.Errorf("definitions = %v, want none", decoded.Definitions)
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	return out
}
