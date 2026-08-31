// Package grouping reports an exported symbol in a file not named for it.
//
// It is a port of the gofsck analyzer of the same name, reimplemented against
// the splint model: the check is the same idea and the reading is different,
// because a document is not a syntax tree.
package grouping

import (
	"context"
	"fmt"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// Name is how the linter is selected and how its issues are labelled.
const Name = "grouping"

// RuleFilename is the one rule this linter reports under.
const RuleFilename = "filename"

// Linter reports an exported symbol in a file not named for it.
type Linter struct{}

// New returns the linter.
func New() *Linter {
	return &Linter{}
}

// Name is the linter name.
func (l *Linter) Name() string {
	return Name
}

// Lint reports the exported symbols of every package outside main and the test
// packages.
//
// A command exports nothing anyone imports, so where it puts a symbol is
// nobody's business, and a test package is not a surface a reader navigates by
// filename either.
func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	var results Results

	for _, def := range root.Packages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if def.Package.Package == "main" || def.Package.TestPackage {
			continue
		}

		symbols := collect(def)
		metric := results.count(def.Package, len(symbols))

		for _, sym := range symbols {
			expected, total, matched := sym.match()
			if matched {
				continue
			}

			results.add(metric, Result{
				Rule:     RuleFilename,
				Symbol:   sym.String(),
				Position: sym.position,
				Message: fmt.Sprintf("exported %s %q expected in [%s] (total: %d expected filenames)",
					sym.kind, sym.String(), strings.Join(expected, ", "), total),
			})
		}
	}

	return results, nil
}
