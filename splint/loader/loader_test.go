package loader_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/titpetric/tools/splint/loader"
	"github.com/titpetric/tools/splint/model"
)

// document is a small document with one package holding one func, which is
// enough to see every stage of a round trip.
func document() *model.DocumentRoot {
	doc := model.NewDocumentRoot("/src", "ast")
	doc.AddModule(&model.Module{Path: "example.com/x", GoVersion: "1.27.0"})
	doc.Packages = model.DefinitionList{{
		Package: model.Package{
			ID:         "example.com/x",
			Package:    "x",
			ImportPath: "example.com/x",
			Path:       ".",
		},
		Imports: model.StringSet{"x.go": []string{`"fmt"`}},
		Funcs: model.DeclarationList{{
			Kind: model.FuncKind, Name: "Open", File: "x.go", Line: 12,
			Signature: "Open (name string) error",
		}},
	}}
	return doc
}

func TestFormatOf(t *testing.T) {
	tests := map[string]loader.Format{
		"go-fsck.json": loader.FormatJSON,
		"model.yml":    loader.FormatYAML,
		"model.YAML":   loader.FormatYAML,
		"model":        loader.FormatJSON,
		"model.txt":    loader.FormatJSON,
	}

	for name, want := range tests {
		if got := loader.FormatOf(name); got != want {
			t.Errorf("FormatOf(%q) = %q, want %q", name, got, want)
		}
	}
}

// TestWriteRoundTrip covers both encodings, which have to describe the same
// document: the model carries a yaml tag per json name for exactly this.
func TestWriteRoundTrip(t *testing.T) {
	for _, format := range []loader.Format{loader.FormatJSON, loader.FormatYAML} {
		var buf bytes.Buffer
		assert.NoError(t, loader.Write(&buf, document(), format), format)

		got, err := loader.Decode(buf.Bytes(), format)
		assert.NoError(t, err, format)
		assert.Equal(t, model.SchemaVersion, got.SchemaVersion, format)
		assert.Equal(t, "/src", got.Root, format)
		assert.Len(t, got.Modules, 1, format)
		assert.Len(t, got.Packages, 1, format)

		pkg := got.Packages[0]
		assert.Equal(t, "example.com/x", pkg.ImportPath, format)
		assert.Len(t, pkg.Funcs, 1, format)
		assert.Equal(t, "Open", pkg.Funcs[0].Name, format)

		// Fill rebuilds what the encoding leaves out.
		assert.Equal(t, []string{`"fmt"`}, pkg.Funcs[0].Imports, format)
	}
}

// TestDecodeBareList covers a go-fsck.json written before there was a root.
func TestDecodeBareList(t *testing.T) {
	data := []byte(`[{"ID":"example.com/x","Package":"x","ImportPath":"example.com/x","Path":".","TestPackage":false,"Module":{"Path":"example.com/x"}}]`)

	got, err := loader.Decode(data, loader.FormatJSON)
	assert.NoError(t, err)
	assert.Equal(t, model.SchemaVersion, got.SchemaVersion)
	assert.Len(t, got.Packages, 1)
	// The module repeated on every package is lifted to the root.
	assert.Len(t, got.Modules, 1)
	assert.Equal(t, "example.com/x", got.Modules[0].Path)
}

func TestLoadAndSave(t *testing.T) {
	for _, name := range []string{"doc.json", "doc.yml"} {
		path := filepath.Join(t.TempDir(), name)
		assert.NoError(t, loader.Save(path, document()), name)

		got, err := loader.Load(path)
		assert.NoError(t, err, name)
		assert.Len(t, got.Packages, 1, name)
	}

	_, err := loader.Load(filepath.Join(t.TempDir(), "missing.json"))
	assert.Error(t, err)
}
