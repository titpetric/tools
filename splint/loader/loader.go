// Package loader reads a parsed document back from disk.
//
// A document written by a parser is the input a linter runs against, which is
// what lets a slow parse be done once and linted many times, and what lets a
// document produced elsewhere be linted here.
package loader

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/titpetric/tools/splint/model"
)

// Format is the encoding of a document.
type Format string

const (
	FormatJSON Format = "json"
	FormatYAML Format = "yaml"
)

// FormatOf returns the format a filename is written in, by extension. Anything
// unrecognised is JSON, which is what the parsers write.
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

// Decode reads a document from bytes.
//
// A document written before there was a root is a bare list of packages, which
// is what "go-fsck extract" writes; it is read as a document holding those
// packages and nothing else, so a file already on disk still loads.
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

// Write encodes a document. JSON is written indented, which is what makes two
// documents of the same tree diffable line by line.
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
// declaration import list: it is derived from the package import set, so it is
// left out of the encoding and rebuilt on the way back in.
func fill(doc *model.DocumentRoot) *model.DocumentRoot {
	for _, def := range doc.Packages {
		def.Fill()
	}
	return doc
}
