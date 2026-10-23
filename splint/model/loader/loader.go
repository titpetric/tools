// Package loader reads a parsed document back from disk.
//
// A document without a root is a bare list of packages, the shape "go-fsck
// extract" writes, and Decode reads it as a document holding those packages.
//
// A document a parser writes is the input a linter runs against, so a slow
// parse runs once and the linters run over its output many times, and the
// linters here run over a document produced elsewhere.
package loader

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/titpetric/tools/splint/model"
)

// Format is the encoding of a document.
type Format string

// The formats a document encodes to.
const (
	FormatJSON Format = "json"
	FormatYAML Format = "yaml"
)

// FormatOf returns the format a filename names, by extension. Anything
// unrecognised is JSON, the format the parsers write.
func FormatOf(filename string) Format {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".yml", ".yaml":
		return FormatYAML
	default:
		return FormatJSON
	}
}

// Load reads a document from a file, in the format its extension names.
func Load(filename string) (*model.DocumentRoot, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return Decode(data, FormatOf(filename))
}

// Decode reads a document from bytes, a rooted document or a bare package list,
// or fails with the error of reading it.
func Decode(data []byte, format Format) (*model.DocumentRoot, error) {
	unmarshal := json.Unmarshal
	if format == FormatYAML {
		unmarshal = yaml.Unmarshal
	}

	var doc model.DocumentRoot
	if err := unmarshal(data, &doc); err == nil && doc.SchemaVersion > 0 {
		return fill(&doc), nil
	}

	var packages model.DefinitionList
	if err := unmarshal(data, &packages); err != nil {
		return nil, fmt.Errorf("read document: %w", err)
	}

	doc = model.DocumentRoot{SchemaVersion: model.SchemaVersion, Packages: packages}
	for _, def := range packages {
		doc.AddModule(def.Module)
	}
	return fill(&doc), nil
}

// Save writes a document to a file, in the format its extension names.
func Save(filename string, doc *model.DocumentRoot) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return Write(file, doc, FormatOf(filename))
}

// Write encodes a document. Write indents JSON, so a line diff of two
// documents of the same tree reads cleanly.
func Write(w io.Writer, doc *model.DocumentRoot, format Format) error {
	if format == FormatYAML {
		encoder := yaml.NewEncoder(w)
		defer encoder.Close()
		return encoder.Encode(doc)
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(doc)
}

// fill recomputes what the document does not carry, which is the per
// declaration import list: the package import set determines it, so the
// encoding leaves it out and fill rebuilds it on the way back in.
func fill(doc *model.DocumentRoot) *model.DocumentRoot {
	for _, def := range doc.Packages {
		def.Fill()
	}
	return doc
}
