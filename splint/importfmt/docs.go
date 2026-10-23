// Package importfmt is the house rule for an import block, written as a
// function of the imports and nothing else.
//
// It reads no files and holds no state. The linter calls it to find out what a
// file's import declaration should look like, and the fixer calls it to write
// that. One function answering both keeps a rule that reports an error and a
// rewrite that clears it from disagreeing.
//
// Normalize writes one import the way the house writes it. An alias repeating
// the last segment of the path says nothing the path does not, and comes off.
// Code reaches a path ending in a major version by a name the path does not
// spell, so it gets one. Canonical sets Break on the first spec of every group
// after the first, so a caller rendering the result puts the blank lines
// where the groups end.
//
// Render is the half of Format that does no deciding. A caller holding the
// imports a file writes renders them with Render and compares the result
// against Format of the same imports: the two strings differ exactly when the
// file does not follow the rule. Decision.Changed compares two renderings of
// the same shape, and never the file on disk against a string, so it answers
// the same whichever parser read the tree. Whitespace inside a declaration is
// gofmt's business; the order of the imports, the groups, the aliases and the
// number of declarations are this package's.
//
// Decide reads the document for the test half of a package as well, which is
// a definition of its own holding only the test files. Without it a white box
// test reaching a name the package declares beside it reads as a missing
// import. Project, the shortest module path of a document, is the project
// group for a tree whose configuration names none: a nested module sits
// under the one at the root of the tree.
package importfmt
