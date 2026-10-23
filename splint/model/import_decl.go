package model

import (
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// ImportSpec is one import as the file writes it.
//
// A formatter has to be able to put this import back. Definition.Imports
// carries the same imports in a form a linter reads them by name, sorted and
// with a redundant alias dropped; nothing in that form says where the import
// sits or what sits beside it.
type ImportSpec struct {
	// Name is the alias in front of the path: empty for an import reached by
	// the base name of its path, "_" for a blank import, "." for a dot
	// import, and a name for anything else.
	Name string `json:"Name,omitempty" yaml:"Name,omitempty"`

	// Path is the import path, without the quotes.
	Path string `json:"Path" yaml:"Path"`

	// Doc is the comment lines directly above the spec, markers trimmed, and
	// Comment is the comment after it on the same line.
	Doc     string `json:"Doc,omitempty" yaml:"Doc,omitempty"`
	Comment string `json:"Comment,omitempty" yaml:"Comment,omitempty"`

	// Line is the 1 indexed line the spec is on.
	Line int `json:"Line,omitempty" yaml:"Line,omitempty"`

	// Break reports a blank line between this spec and the one before it;
	// blank lines group the imports of a file.
	Break bool `json:"Break,omitempty" yaml:"Break,omitempty"`
}

// Literal is the spec as Definition.Imports records it: the quoted path, with
// the alias and a space in front of it when there is one.
func (s ImportSpec) Literal() string {
	quoted := strconv.Quote(s.Path)
	if s.Name == "" {
		return quoted
	}
	return s.Name + " " + quoted
}

// Blank reports an import written for its side effect alone.
func (s ImportSpec) Blank() bool {
	return s.Name == "_"
}

// Dot reports an import that puts the package's names in the file's scope.
func (s ImportSpec) Dot() bool {
	return s.Name == "."
}

// Ref is "" for a blank or dot import,
// the base name of the path without an alias,
// and the alias otherwise.
func (s ImportSpec) Ref() string {
	switch s.Name {
	case "_", ".":
		return ""
	case "":
		return BaseName(s.Path)
	}
	return s.Name
}

// IsPackageName reports a string Go would accept as a package name.
func IsPackageName(name string) bool {
	if name == "" {
		return false
	}

	for i, r := range name {
		switch {
		case r == '_' || unicode.IsLetter(r):
		case i > 0 && unicode.IsDigit(r):
		default:
			return false
		}
	}

	return true
}

// BaseName is the last segment of a path, its major version taken off.
func BaseName(path string) string {
	trimmed := majorVersion.ReplaceAllString(path, "")
	if index := strings.LastIndexByte(trimmed, '/'); index >= 0 {
		trimmed = trimmed[index+1:]
	}
	return dottedVersion.ReplaceAllString(trimmed, "")
}

// ImportDecl is one import declaration of a file, as the file spells it. A
// declaration importing "C" carries the cgo preamble and stays in place.
type ImportDecl struct {
	// Line is the 1 indexed line the import keyword is on, and EndLine the
	// line the declaration ends on: the closing paren for a block, and Line
	// itself for a single import.
	Line    int `json:"Line" yaml:"Line"`
	EndLine int `json:"EndLine" yaml:"EndLine"`

	// Block reports a declaration written as import ( ... ), and false for a
	// single import on one line.
	Block bool `json:"Block,omitempty" yaml:"Block,omitempty"`

	// Doc is the comment lines directly above the declaration, markers
	// trimmed, and DocLine is the first of them. A declaration merged into
	// another takes its doc with it: leaving it behind attaches it to whatever
	// declaration follows, which is a comment about the imports now reading as
	// a comment about a type.
	Doc     string `json:"Doc,omitempty" yaml:"Doc,omitempty"`
	DocLine int    `json:"DocLine,omitempty" yaml:"DocLine,omitempty"`

	// Specs are the imports of the declaration, in source order.
	Specs []ImportSpec `json:"Specs,omitempty" yaml:"Specs,omitempty"`

	// Tail is the comment lines inside the declaration that belong to no
	// import, markers trimmed: the ones after the last one, and any separated
	// from the import below them by a blank line.
	//
	// The parse records them because a rewrite without them would delete
	// what somebody wrote. A formatter may reorder a file and may not lose a
	// line of it.
	Tail string `json:"Tail,omitempty" yaml:"Tail,omitempty"`
}

// Cgo reports a declaration that imports "C",
// and false for any other.
func (d ImportDecl) Cgo() bool {
	for _, spec := range d.Specs {
		if spec.Path == "C" {
			return true
		}
	}
	return false
}

// ImportDeclList is the import declarations of one file, in the order they
// appear. A file writing two declarations reaches the specs of both.
type ImportDeclList []ImportDecl

// Free is nil for no declarations,
// and every one not importing "C" otherwise.
func (l ImportDeclList) Free() ImportDeclList {
	if len(l) == 0 {
		return nil
	}

	free := make(ImportDeclList, 0, len(l))
	for _, decl := range l {
		if !decl.Cgo() {
			free = append(free, decl)
		}
	}

	return free
}

// Specs is every spec of every declaration, in source order.
func (l ImportDeclList) Specs() []ImportSpec {
	var specs []ImportSpec
	for _, decl := range l {
		specs = append(specs, decl.Specs...)
	}
	return specs
}

// Tail is the comments a merge of the declarations keeps, see the package doc.
func (l ImportDeclList) Tail() string {
	var tails []string

	for i, decl := range l {
		if i > 0 && decl.Doc != "" {
			tails = append(tails, decl.Doc)
		}
		if decl.Tail != "" {
			tails = append(tails, decl.Tail)
		}
	}

	return strings.Join(tails, "\n")
}

// Named returns the spec reached by a name, and whether the file has one.
func (l ImportDeclList) Named(name string) (ImportSpec, bool) {
	for _, decl := range l {
		for _, spec := range decl.Specs {
			if spec.Ref() == name {
				return spec, true
			}
		}
	}
	return ImportSpec{}, false
}

// Refs are the sorted names the file reaches its imports by.
func (l ImportDeclList) Refs() []string {
	var names []string
	for _, spec := range l.Specs() {
		if ref := spec.Ref(); ref != "" {
			names = append(names, ref)
		}
	}
	sort.Strings(names)
	return names
}
