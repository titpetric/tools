package model

// SchemaVersion is the version of the document a parser writes. A reader that
// finds a higher one is looking at a document it does not fully understand.
const SchemaVersion = 1

// DocumentRoot is everything one parse produced: the packages it found and the
// modules they belong to.
//
// A parser fills it and a linter reads it. Nothing here is Go specific: a
// parser for another language fills the same document, so the linters port to
// it.
type DocumentRoot struct {
	// SchemaVersion is the version of the schema the document follows.
	SchemaVersion int `json:"SchemaVersion" yaml:"SchemaVersion"`

	// Root is the directory the parse started at, so a reader can resolve a
	// file path in the document back to disk.
	Root string `json:"Root" yaml:"Root"`

	// Parser names what produced the document, "ast" or "simple"; that tells
	// two documents of the same tree apart.
	Parser string `json:"Parser,omitempty" yaml:"Parser,omitempty"`

	// Modules are the go.mod files found under Root, each recorded once.
	Modules []*Module `json:"Modules,omitempty" yaml:"Modules,omitempty"`

	// Packages are the packages the parse found.
	Packages DefinitionList `json:"Packages" yaml:"Packages"`

	// Tagged are the test files a build constraint keeps out of the build,
	// read the same way as the rest: one test definition per directory,
	// whose Excluded names the constraint of each file. The linters that
	// count tests read them through WithTagged; no other reads them. The
	// parse fills them when its options include the test facts.
	Tagged DefinitionList `json:"Tagged,omitempty" yaml:"Tagged,omitempty"`
}

// NewDocumentRoot returns a document ready for a parser to fill.
func NewDocumentRoot(root, parser string) *DocumentRoot {
	return &DocumentRoot{
		SchemaVersion: SchemaVersion,
		Root:          root,
		Parser:        parser,
		Packages:      DefinitionList{},
	}
}

// AddModule records a go.mod, unless the same path is already recorded.
func (d *DocumentRoot) AddModule(module *Module) {
	if module == nil {
		return
	}
	for _, known := range d.Modules {
		if known.Path == module.Path {
			return
		}
	}
	d.Modules = append(d.Modules, module)
}

// Sort orders the packages by import path, and the declarations within each,
// so two parses of the same tree produce the same document.
func (d *DocumentRoot) Sort() {
	for _, def := range d.Packages {
		def.Sort()
	}
	d.Packages.Sort()
}
