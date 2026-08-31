package imports

import (
	"iter"

	"github.com/titpetric/tools/splint/model"
)

// Result is one import collision, in the terms this linter thinks in.
type Result struct {
	// Rule is which of the import rules the finding is under.
	Rule string

	// Position is the package the collision is in, which has no line: the
	// collision is between two files and belongs to neither.
	Position model.Position

	// Message names the two import paths that collided.
	Message string
}

// Issue renders the finding as the framework reads it.
func (r Result) Issue() model.Issue {
	return model.Issue{
		Linter:   Name,
		Rule:     r.Rule,
		Severity: model.SeverityWarn,
		Position: r.Position,
		Message:  r.Message,
	}
}

// Results is what the linter found, and is what the framework ranges over.
type Results []Result

// Linter names the linter the report came from.
func (r Results) Linter() string {
	return Name
}

// Len is how many findings there are.
func (r Results) Len() int {
	return len(r)
}

// All yields every finding as an issue.
func (r Results) All() iter.Seq[model.Issue] {
	return func(yield func(model.Issue) bool) {
		for _, result := range r {
			if !yield(result.Issue()) {
				return
			}
		}
	}
}
