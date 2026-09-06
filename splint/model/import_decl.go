package model

import (
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// ImportSpec is one import as the file writes it.
//
// It is the import a formatter has to be able to put back. Definition.Imports
// carries the same imports in a form a linter reads them by name, sorted and
// with a redundant alias dropped; nothing in that form says where the import
// was written or what was written beside it.
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

	// Break reports a blank line between this spec and the one before it,
	// which is how the file groups its imports.
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

// Ref is the name the file reaches the package by, and is empty for a blank or
// a dot import, which no name reaches.
//
// An unaliased path is reached by its last segment, with a major version
// suffix taken off: "example.com/x/v2" is reached as x.
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
//
// The name a path implies is a guess, and not every path implies one: the last
// segment of "github.com/goccy/go-yaml" is not an identifier, and neither is
// what is left of "example.com/v2" once the version comes off. Writing either
// as an alias produces a file that does not parse.
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

// BaseName is the name a package path is reached by without an alias: the last
// segment, with a major version suffix taken off.
//
// A version is written two ways. The module convention puts it in its own
// segment, so "example.com/x/v2" is reached as x. gopkg.in puts it on the
// segment, so "gopkg.in/yaml.v3" is reached as yaml.
func BaseName(path string) string {
	trimmed := majorVersion.ReplaceAllString(path, "")
	if index := strings.LastIndexByte(trimmed, '/'); index >= 0 {
		trimmed = trimmed[index+1:]
	}
	return dottedVersion.ReplaceAllString(trimmed, "")
}

// ImportDecl is one import declaration of a file, as it is written.
type ImportDecl struct {
	// Line is the 1 indexed line the import keyword is on, and EndLine the
	// line the declaration ends on: the closing paren for a block, and Line
	// itself for a single import.
	Line    int `json:"Line" yaml:"Line"`
	EndLine int `json:"EndLine" yaml:"EndLine"`

	// Block reports a declaration written as import ( ... ) rather than as a
	// single import on one line.
	Block bool `json:"Block,omitempty" yaml:"Block,omitempty"`

	// Doc is the comment lines directly above the declaration, markers
	// trimmed, and DocLine is the first of them. A declaration merged into
	// another takes its doc with it: leaving it behind attaches it to whatever
	// declaration follows, which is a comment about the imports now reading as
	// a comment about a type.
	Doc     string `json:"Doc,omitempty" yaml:"Doc,omitempty"`
	DocLine int    `json:"DocLine,omitempty" yaml:"DocLine,omitempty"`

	// Specs are the imports of the declaration, in the order they are written.
	Specs []ImportSpec `json:"Specs,omitempty" yaml:"Specs,omitempty"`

	// Tail is the comment lines inside the declaration that belong to no
	// import, markers trimmed: the ones after the last one, and any separated
	// from the import below them by a blank line.
	//
	// They are recorded because a rewrite that did not would be deleting what
	// somebody wrote. A formatter may reorder a file and may not lose a line
	// of it.
	Tail string `json:"Tail,omitempty" yaml:"Tail,omitempty"`
}

// Cgo reports a declaration that imports "C".
//
// The comment above such a declaration is the C source cgo compiles, and it is
// the preamble only while it sits directly above the import. The declaration
// is where it is because of what is written above it, so nothing may move it.
func (d ImportDecl) Cgo() bool {
	for _, spec := range d.Specs {
		if spec.Path == "C" {
			return true
		}
	}
	return false
}

// ImportDeclList is the import declarations of one file, in the order they
// appear.
type ImportDeclList []ImportDecl

// Free are the declarations a formatter may rewrite, which is every one that
// does not import "C".
//
// A cgo file is formatted like any other apart from that one declaration: the
// other blocks in it are merged, grouped and sorted, and the import of "C"
// stays where its preamble put it.
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

// Specs is every spec of every declaration, in the order they are written.
//
// A file writing two import declarations reaches all of them, so a reader
// asking what the file imports asks this rather than walking the declarations.
func (l ImportDeclList) Specs() []ImportSpec {
	var specs []ImportSpec
	for _, decl := range l {
		specs = append(specs, decl.Specs...)
	}
	return specs
}

// Tail is the comment lines of every declaration that belong to no import,
// which is where they go when two declarations merge into one.
//
// The doc comment of every declaration but the first is in here too. The first
// declaration keeps its own, because the block is written where that one was
// and the comment above it is still above it; the others are written away, and
// their comments come along rather than being left to attach themselves to the
// next thing in the file.
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

// Refs are the names the file reaches its imports by, in order, leaving out
// the blank and dot imports that no name reaches.
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
