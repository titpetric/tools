package filecheck

import (
	"github.com/titpetric/tools/splint/pathmatch"
)

// Allowance is a ceiling a tree stated for the files one pattern covers.
//
// It exists because the rule is about a file with one job and the number is
// a proxy for it. A recursive descent parser and a bytecode interpreter loop
// each do one job at a length no general number can name, and splitting one
// on a line a linter picked is a worse file than the long one. Naming the path
// in a committed file says which files those are, and leaves a new long file
// reported.
type Allowance struct {
	// Path is the pattern the ceiling applies to, matched by pathmatch
	// against the file as a report names it: the package directory and the
	// filename, with no leading dot or slash. The syntax is path.Match per
	// segment, where a star stops at a separator, plus ** for any number of
	// segments, so parser/*.go is one directory and **/*_test.go is every
	// test file of the tree.
	Path string

	// MaxLines is the ceiling for the files it covers. Zero means the rule
	// does not apply to them, the way a zero turns off the pollution rules.
	MaxLines int
}

// Allowances are the ceilings a tree stated, in the order it stated them. The
// first entry covering a file sets its ceiling: an author lists a narrow entry
// above the broad one it carves a hole in, and the order states which glob
// applies, as with parser/*.go and **/*_test.go.
type Allowances []Allowance

// MaxLinesFor is the ceiling of the first entry covering path, or fallback.
// A malformed pattern, which the loader reports, matches nothing.
func (a Allowances) MaxLinesFor(path string, fallback int) int {
	for _, entry := range a {
		matched, err := pathmatch.Match(entry.Path, path)
		if err == nil && matched {
			return entry.MaxLines
		}
	}
	return fallback
}
