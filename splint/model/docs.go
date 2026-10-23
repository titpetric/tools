// Package model is the document a parse of a Go tree produces: packages,
// their files and declarations, and what each declaration reaches.
//
// # Import paths and names
//
// A file reaches an unaliased import by the last segment of its path, with
// a major version suffix taken off. A path spells a version two ways: the
// module convention puts it in its own segment, so x reaches
// "example.com/x/v2", and gopkg.in puts it on the segment, so yaml reaches
// "gopkg.in/yaml.v3".
//
// The name a path implies is a guess, and not every path implies one: the
// last segment of "github.com/goccy/go-yaml" is not an identifier, and
// neither is what remains of "example.com/v2" once the version comes off.
// Writing either as an alias produces a file that does not parse, which is
// what IsPackageName guards.
//
// # Import declarations
//
// The comment above a declaration importing "C" is the C source cgo
// compiles, and it stays the preamble only while it sits directly above the
// import. A formatter merges, groups and sorts the other declarations of a
// cgo file, and the import of "C" stays where its preamble put it.
//
// When declarations merge into one, the first keeps its doc comment, because
// the block lands where that one stands. The docs of the others go into the
// tail with the comments that belong to no import, so no comment attaches
// itself to the next thing in the file and a rewrite loses no line.
//
// # Build constraints
//
// Only the lines above the package clause can hold a //go:build line, so
// BuildConstraint stops at the clause. The expression stays text: what it
// means is a question for go/build/constraint, and the model parses nothing.
package model
