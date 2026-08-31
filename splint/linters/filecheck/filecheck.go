// Package filecheck reports a file long enough to be doing more than one
// thing, and measures how the lengths of a tree are spread.
//
// It is the gofsck check of the same name, read against the splint model
// instead of against the disk. gofsck walks the filesystem and buckets every
// file it finds by extension, so markdown and everything else is scored
// alongside the code. A document describes Go packages and nothing else, so
// there is only ever one extension here and the bucketing has nowhere to go.
// What is left is the part that carried the idea: a tree of small files reads
// well, and one holding a few enormous files does not.
package filecheck

import (
	"context"
	"fmt"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// Name is how the linter is selected and how its issues are labelled.
const Name = "filecheck"

// RuleLong is the one rule this linter reports under.
const RuleLong = "long"

// maxLines is how many lines a file may run to before it reads as more than
// one thing. The count has blanks and comments taken out of it already, so
// this is the code itself, and four hundred lines of code is about where a
// reader stops holding a file in their head and starts searching it. A file
// that has to be searched has more than one subject in it. It is also where
// the gofsck size histogram turns: its "high" band tops out around five
// hundred lines counted with the blanks and the comments still in.
const maxLines = 400

// Linter checks the length of every file.
type Linter struct{}

// New returns the linter.
func New() *Linter {
	return &Linter{}
}

// Name is the linter name.
func (l *Linter) Name() string {
	return Name
}

// Lint measures every file of every package and reports the long ones.
//
// Generated files are measured by nobody: their length says something about
// the generator and nothing about the code, and a protobuf stub would drown
// the histogram on its own.
func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	var results Results

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
			if metric == nil || file.Lines <= maxLines {
				continue
			}

			results.add(metric, Result{
				Rule:     RuleLong,
				Symbol:   file.Name,
				Position: model.Position{Package: def.Package.Package, File: path},
				Message: fmt.Sprintf("file is %d lines of code, over the %d a file doing one thing runs to",
					file.Lines, maxLines),
			})
		}
	}

	return results, nil
}

// filePath names a file the way a Position names one, the package directory
// and the filename. It is Declaration.Position for a file rather than for a
// symbol in it, and it carries no line: the finding is about the whole file.
func filePath(pkg model.Package, file model.File) string {
	dir := strings.TrimPrefix(strings.TrimPrefix(pkg.Path, "."), "/")
	if dir == "" {
		return file.Name
	}
	return dir + "/" + file.Name
}
