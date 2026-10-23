// Package filecheck reports a file long enough to be doing more than one
// thing, and measures the spread of lengths across a tree.
//
// The package carries the gofsck check of the same name, reading the splint model
// where gofsck reads the disk. gofsck walks the filesystem and buckets every
// file it finds by extension, so it scores markdown and everything else
// alongside the code. A document describes Go packages and nothing else, so
// there is only ever one extension here and the bucketing has nowhere to go.
// What remains is the part that carried the idea: a tree of small files reads
// well, and one holding a few enormous files does not.
package filecheck

import (
	"context"
	"fmt"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// Name selects the linter and labels its issues.
const Name = "filecheck"

// RuleLong is the one rule this linter reports under.
const RuleLong = "long"

// DefaultMaxLines is how many lines a file may run to before it reads as more
// than one job. The count has blanks and comments taken out of it already,
// so this is the code itself, and four hundred lines of code is about where a
// reader stops holding a file in their head and starts searching it. A file
// that needs a search has more than one subject in it. The number also marks where
// the gofsck size histogram turns: its "high" band tops out around five
// hundred lines counted with the blanks and the comments still in.
//
// A tree with no filecheck section in its .splint.yml runs under this number,
// so the configuration names it and carries no second four hundred.
const DefaultMaxLines = 400

// Linter checks the length of every file against the ceiling it runs under, so
// a file the tree gave room to counts into the histogram and stays out of the
// findings. It measures no generated file, whose length speaks for the
// generator, and a protobuf stub would drown the histogram on its own.
type Linter struct {
	// MaxLines is the ceiling for a file no Allow entry covers. Zero turns
	// the rule off for those files.
	MaxLines int

	// Allow are the ceilings a tree stated for the paths that earn one.
	Allow Allowances
}

// New returns the linter under the default ceiling and no allowances.
func New() *Linter {
	return &Linter{MaxLines: DefaultMaxLines}
}

// Name is the linter name.
func (l *Linter) Name() string {
	return Name
}

// Lint measures every file of every package and reports the long ones.
func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	results := Results{max: l.MaxLines, tuned: len(l.Allow) > 0}

	for _, def := range root.Packages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		for _, file := range def.Files {
			if file.Generated {
				continue
			}

			path := filePath(def.Package, file)
			metric := results.count(path, file)
			if metric == nil {
				continue
			}

			ceiling := l.Allow.MaxLinesFor(path, l.MaxLines)
			if ceiling <= 0 || file.Lines <= ceiling {
				continue
			}

			results.add(metric, Result{
				Rule:     RuleLong,
				Symbol:   file.Name,
				Position: model.Position{Package: def.Package.Package, File: path},
				Message: fmt.Sprintf("file is %d lines of code, over the %d a file doing one thing runs to",
					file.Lines, ceiling),
			})
		}
	}

	return results, nil
}

// filePath names a file the way a Position names one, the package directory
// and the filename. Declaration.Position names a symbol; this names a whole
// file and carries no line: the finding is about the whole file.
func filePath(pkg model.Package, file model.File) string {
	dir := strings.TrimPrefix(strings.TrimPrefix(pkg.Path, "."), "/")
	if dir == "" {
		return file.Name
	}
	return dir + "/" + file.Name
}
