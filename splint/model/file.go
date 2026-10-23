package model

import (
	"strings"
)

// File is one file of a package, as the parse read it.
//
// Without this, a file that declares nothing stays invisible to the model: a
// declaration records the file holding it and the import set uses the
// filename as its key, and neither says anything about a file holding
// neither. A check that counts files, or pairs one with its test, has to be
// able to see them all.
type File struct {
	// Name is the base filename, the same one a declaration records.
	Name string `json:"Name" yaml:"Name"`

	// Lines is the code in the file, blanks and comments left out, and Size
	// is the bytes it takes on disk.
	Lines int `json:"Lines" yaml:"Lines"`
	Size  int `json:"Size" yaml:"Size"`

	// Generated reports the "Code generated ... DO NOT EDIT." marker, which
	// is a file nobody wrote and no check should judge.
	Generated bool `json:"Generated,omitempty" yaml:"Generated,omitempty"`

	// Test reports a file the toolchain compiles into the test binary.
	Test bool `json:"Test,omitempty" yaml:"Test,omitempty"`

	// Directives reports a file carrying //line directives, which rebase the
	// line numbers the compiler reports from that point on.
	//
	// The physical lines of such a file carry positions: moving a directive,
	// or deleting the declaration one sits above, changes every position
	// reported for the rest of the file. Nothing here rewrites one. They are
	// machine output, from goyacc and its like, and no reader wrote them.
	Directives bool `json:"Directives,omitempty" yaml:"Directives,omitempty"`

	// Package is the name in the file's own package clause.
	//
	// It can differ from the name of the package the parse records the file
	// under. A directory holds at most one package and up to two test
	// scopes, and the external one declares "<name>_test": the two are one
	// definition in the model and only the clause tells a file of one from a
	// file of the other.
	// A file may not import the package its clause names, and the external
	// test file must.
	Package string `json:"Package,omitempty" yaml:"Package,omitempty"`

	// ImportDecls are the import declarations of the file, in source order.
	// The parse fills them when its options include them, and a document
	// written without them describes the same package.
	ImportDecls ImportDeclList `json:"ImportDecls,omitempty" yaml:"ImportDecls,omitempty"`

	// Uses are the identifiers the file writes before a dot, sorted and
	// deduplicated: the names an import of this file may have to answer to.
	//
	// It over-collects. A local variable, a parameter and a receiver are all
	// in it, so "t" from t.Run is a use. That is the direction that cannot
	// break a build: a name in here keeps an import that is not needed, and a
	// name missing from here removes one that is.
	Uses []string `json:"Uses,omitempty" yaml:"Uses,omitempty"`
}

// HasLineDirectives reports a line opening on //line or /*line,
// and false for source without one.
func HasLineDirectives(lines []string) bool {
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "//line ") || strings.HasPrefix(trimmed, "/*line ") {
			return true
		}
	}
	return false
}

// BuildConstraint returns "" at the package clause,
// the //go:build expression as the file writes it,
// or "" for a file with none.
func BuildConstraint(source []byte) string {
	for _, line := range strings.Split(string(source), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			return ""
		}
		if expr, ok := strings.CutPrefix(trimmed, "//go:build "); ok {
			return strings.TrimSpace(expr)
		}
	}
	return ""
}

// UsesName reports whether the file writes the name before a dot.
func (f File) UsesName(name string) bool {
	for _, use := range f.Uses {
		if use == name {
			return true
		}
	}
	return false
}

// Base is the filename without .go and _test, pairing a file with its test.
func (f File) Base() string {
	name := strings.TrimSuffix(f.Name, ".go")
	return strings.TrimSuffix(name, "_test")
}

// FileList is the files of one package.
type FileList []File

// Find returns the file of a name, and whether the package holds one.
func (l FileList) Find(name string) (File, bool) {
	for _, file := range l {
		if file.Name == name {
			return file, true
		}
	}
	return File{}, false
}

// Lines is the code of every file together.
func (l FileList) Lines() int {
	total := 0
	for _, file := range l {
		total += file.Lines
	}
	return total
}

// Filter returns the files a test accepts.
func (l FileList) Filter(match func(File) bool) FileList {
	var kept FileList
	for _, file := range l {
		if match(file) {
			kept = append(kept, file)
		}
	}
	return kept
}
