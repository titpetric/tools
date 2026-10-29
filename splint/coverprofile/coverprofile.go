// Package coverprofile folds a Go coverage profile into a parsed document.
//
// The profile is the file "go test -coverprofile" writes: a mode line and one
// line per block, naming the file by import path and the block by the lines and
// columns it spans, with the statements in it and the times it ran. The model
// already carries a place for what it says, on model.Complexity, beside the
// complexity measured over the same declaration.
//
// It replaces a summary of the profile taken by a second tool. Reading the
// profile directly makes the coverage part of extracting a document, with no
// separate step after it.
package coverprofile

import (
	"fmt"

	"golang.org/x/tools/cover"

	"github.com/titpetric/tools/splint/model"
)

// Apply reads the profile at filename and writes what it says onto the
// document.
func Apply(root *model.DocumentRoot, filename string) error {
	profiles, err := cover.ParseProfiles(filename)
	if err != nil {
		return fmt.Errorf("read coverage profile %s: %w", filename, err)
	}

	Overlay(root, profiles)
	return nil
}

// Overlay writes the share of statements that ran onto every function and package,
// and sets each package to the summed complexity of its declarations.
func Overlay(root *model.DocumentRoot, profiles []*cover.Profile) {
	if root == nil {
		return
	}

	blocks := index(profiles)

	for _, def := range root.Packages {
		if def == nil {
			continue
		}

		total := statements{}
		summed := model.Complexity{}

		for _, decl := range def.Funcs {
			if decl.Complexity == nil {
				continue
			}

			summed.Cognitive += decl.Complexity.Cognitive
			summed.Cyclomatic += decl.Complexity.Cyclomatic
			summed.Lines += decl.Complexity.Lines

			counted := count(blocks[key(def.Package.ImportPath, decl.File)], decl.Line, decl.Line+decl.Complexity.Lines-1)
			decl.Complexity.Coverage = counted.percent()

			total.covered += counted.covered
			total.total += counted.total
		}

		summed.Coverage = total.percent()
		def.Package.Complexity = &summed
	}
}

// index keys the blocks of a profile by the file the profile measured them
// in. A profile names a file by its import path and base name, and a package
// and a declaration of the document spell the same pair between them.
func index(profiles []*cover.Profile) map[string][]cover.ProfileBlock {
	out := make(map[string][]cover.ProfileBlock, len(profiles))
	for _, profile := range profiles {
		if profile == nil {
			continue
		}
		out[profile.FileName] = append(out[profile.FileName], profile.Blocks...)
	}
	return out
}

// key names a file the way a profile names it.
func key(importPath, file string) string {
	if importPath == "" {
		return file
	}
	return importPath + "/" + file
}

// count adds up the blocks that fall inside a line range.
//
// A block is inside when it starts inside: a declaration spans from its func
// keyword to the brace that closes it, so every block of its body opens within
// those lines and no block of another declaration does.
func count(blocks []cover.ProfileBlock, from, to int) statements {
	var out statements

	for _, block := range blocks {
		if block.StartLine < from || block.StartLine > to {
			continue
		}
		out.total += block.NumStmt
		if block.Count > 0 {
			out.covered += block.NumStmt
		}
	}

	return out
}

// statements is what a profile counts: the statements measured, and the ones
// that ran.
type statements struct {
	covered int
	total   int
}

// percent is the share of the statements that ran, weighted by statement the
// way "go tool cover -func" weights it, and never by block.
//
// Nothing to count reports 0. The cover tool never measures a declaration
// with no statements and reports 0 for it, so a report reads the same
// from either of the two.
func (s statements) percent() float64 {
	if s.total == 0 {
		return 0
	}
	return 100 * float64(s.covered) / float64(s.total)
}
