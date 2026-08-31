// Package imports reports two packages a file reaches under one name, which is
// a name that means one thing in one file and another in the next.
package imports

import (
	"context"

	"github.com/titpetric/tools/splint/model"
)

// Name is how the linter is selected and how its issues are labelled.
const Name = "imports"

// RuleCollision is the one rule this linter reports under.
const RuleCollision = "collision"

// Linter checks the import names of every package.
type Linter struct{}

// New returns the linter.
func New() *Linter {
	return &Linter{}
}

// Name is the linter name.
func (l *Linter) Name() string {
	return Name
}

// Lint reports the import collisions of every package.
//
// A collision is between files rather than within one, which the compiler
// would have caught: two files of the same package reaching different modules
// under the same short name compile and read as though they agree.
func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	var results Results

	for _, def := range root.Packages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		_, collisions := def.Imports.Map(def.Imports.All())
		for _, collision := range collisions {
			results = append(results, Result{
				Rule:     RuleCollision,
				Position: model.Position{Package: def.Package.Package, File: importFile(def)},
				Message:  collision.Error(),
			})
		}
	}

	return results, nil
}

// importFile names the package the collision is in. An import set is keyed on
// the file that declared each import and a collision is between two of them,
// so the package directory is what the two have in common.
func importFile(def *model.Definition) string {
	dir := def.Package.Path
	if dir == "." || dir == "" {
		return "."
	}
	for len(dir) > 0 && (dir[0] == '.' || dir[0] == '/') {
		dir = dir[1:]
	}
	return dir
}
