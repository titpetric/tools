// Package tests holds the harness that compares the two parsers.
//
// The harness lives in a package of its own so a reader can read and reuse the
// comparison: it walks two documents and says where they differ, and that is
// worth having outside the test that runs it.
//
// Diff compares the encoded form deeply and checks no hand-picked list of
// fields: a key that only one side carries is a difference, and so is a key
// added to the model tomorrow. Coverage never depends on an enumeration,
// because the harness enumerates nothing. The normalise function takes a path
// and the two values at it, and returns the values to compare in their place.
// A caller uses it to take a known and explained difference, such as column
// alignment inside a source listing, out of the comparison without taking the
// field out of it.
//
// Canonical turns the declaration lists into maps keyed on the symbol, so a
// comparison lines two documents up by identity: one extra declaration is one
// difference, where a positional walk would shift every declaration after it
// and report a hundred. Everything else stays as it encodes, so the walk
// still covers a key nobody thought about. Field drops the array indices of a
// path, so a count puts every element of a list under the field it belongs to.
package tests
