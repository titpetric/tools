package model

import (
	"path"
	"strconv"
	"strings"
)

// Position names where an issue is.
//
// File is relative to the root the document was parsed at and never begins
// with a separator, which is what makes a rendered line resolve against a
// checkout: "model/declaration.go:42" is a path GitHub can open, "/model/..."
// is not.
type Position struct {
	// Package is the package the issue is in, named the way the document names
	// it.
	Package string `json:"Package,omitempty" yaml:"Package,omitempty"`

	// File is the root relative path of the file.
	File string `json:"File" yaml:"File"`

	// Line is the 1 indexed line, and is zero for an issue about the file or
	// the package rather than a line of it.
	Line int `json:"Line,omitempty" yaml:"Line,omitempty"`

	// EndLine is the last line of the run the issue covers, and is zero for an
	// issue about one line. A rule that reports a block sets it, and an
	// annotation carries it so a review marks the whole block rather than the
	// line it opens on.
	EndLine int `json:"EndLine,omitempty" yaml:"EndLine,omitempty"`
}

// Path is the file as a path a tool resolves against the checkout, without the
// line: it never begins with a separator, because "/model/x.go" is a path no
// checkout holds.
func (p Position) Path() string {
	return strings.TrimPrefix(path.Clean(p.File), "/")
}

// Block is the last line the issue covers, which is the line it opens on for
// an issue that covers one.
func (p Position) Block() int {
	if p.EndLine > p.Line {
		return p.EndLine
	}
	return p.Line
}

// String reads "package: file.go:line", which names the package once and then
// the file within it, rather than repeating the path in both halves.
func (p Position) String() string {
	where := path.Base(p.File)
	if p.Line > 0 {
		where += ":" + strconv.Itoa(p.Line)
	}
	if p.Package == "" {
		return where
	}
	return p.Package + ": " + where
}

// Ref is the position as a path a tool resolves against the checkout, which is
// what a compiler writes and what GitHub Actions reads back.
func (p Position) Ref() string {
	if p.Line > 0 {
		return p.Path() + ":" + strconv.Itoa(p.Line)
	}
	return p.Path()
}
