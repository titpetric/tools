// Package docs renders a parsed document as an API reference.
//
// The reference is the package godoc, the types, consts and vars a package
// declares, and the signature of every exported function. Function bodies are
// not printed; godoc examples are the exception, printed whole from the test
// package they live in.
//
// Markdown is the default rendering. The others are a spec of the declared
// symbols, two plantuml diagrams of what the packages reach, and the document
// itself as JSON. Split mode writes one markdown file per package instead of
// one document on the writer.
package docs

import (
	"encoding/json"
	"io"
	"path"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// The renderings a document is written as.
const (
	FormatMarkdown = "markdown"
	FormatSpec     = "spec"
	FormatImports  = "imports"
	FormatPlantUML = "puml"
	FormatJSON     = "json"
	FormatConfig   = "config"
)

// Options is what one rendering was asked for.
type Options struct {
	// Format names the rendering: markdown, spec, imports, puml or json.
	// Empty means markdown.
	Format string

	// Split writes one markdown file per package under OutDir, with a README
	// listing them, instead of one document on the writer.
	Split  bool
	OutDir string

	// StripPrefix are package prefixes taken off an import path when a split
	// file is named after it. With none given, the first path segment comes
	// off instead.
	StripPrefix []string

	// Model leaves functions and interfaces out of the puml diagram, so what
	// is drawn is the data model alone.
	Model bool

	// Hide are type names left out of the puml diagram.
	Hide []string

	// Root names the type a config reference opens on, and Title replaces
	// its heading. Naming a root selects the config rendering.
	Root  string
	Title string

	// Symbol names one declaration to print alone, godoc style, with how
	// many tests reach it: "Open" or "Client.Close". Naming one overrides
	// the format.
	Symbol string

	// Verbose keeps the unexported symbols in the spec rendering, and lists
	// every reference to a symbol under the symbol rendering.
	Verbose bool
}

// Write renders the document to w in the format the options name.
func Write(w io.Writer, root *model.DocumentRoot, opts Options) error {
	defs := root.Packages

	if opts.Symbol != "" {
		return renderSymbol(w, root, opts)
	}

	if opts.Split {
		return renderSplit(defs, opts)
	}

	switch opts.Format {
	case FormatSpec:
		return renderSpec(w, opts, defs)
	case FormatImports:
		return renderImports(w, defs)
	case FormatJSON:
		return renderJSON(w, defs)
	case FormatPlantUML, "plantuml":
		return renderPlantUML(w, opts, defs)
	case FormatConfig:
		return renderConfig(w, opts, defs)
	default:
		// A root type selects the config reference: it is the one rendering
		// a root means anything to.
		if opts.Root != "" {
			return renderConfig(w, opts, defs)
		}
		return renderMarkdown(w, defs)
	}
}

// renderJSON writes the packages of the document, which is the document as a
// program reads it.
func renderJSON(w io.Writer, defs model.DefinitionList) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(defs)
}

// skipDocs reports a package the reference leaves out, which is a test
// package: the reference documents what a consumer imports, and nobody
// imports one.
func skipDocs(def *model.Definition) bool {
	return def.Package.TestPackage || strings.HasSuffix(def.Package.Package, "_test")
}

// symbol is the line a function is listed as, which is its signature with the
// receiver in front where it has one.
func symbol(fn *model.Declaration) string {
	if fn.Receiver != "" {
		return "func (" + fn.Receiver + ") " + fn.Signature
	}
	return "func " + fn.Signature
}

// packageTitle is what a package is headed as: the path it was parsed under,
// or the base of its import path when it is the root.
func packageTitle(pkg model.Package) string {
	if pkg.Path == "." || pkg.Path == "" {
		return path.Base(pkg.ImportPath)
	}
	return pkg.Path
}
