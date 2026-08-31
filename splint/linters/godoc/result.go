package godoc

import (
	"iter"

	"github.com/titpetric/tools/splint/model"
)

// Result is one godoc finding, in the terms this linter thinks in.
type Result struct {
	// Rule is which of the godoc rules the finding is under.
	Rule string

	// Symbol is the declaration, receiver and name, and Kind is what it
	// declares.
	Symbol string
	Kind   model.DeclarationKind

	// Position is where it is, and Message says what is wrong.
	Position model.Position
	Message  string
}

// Issue renders the finding as the framework reads it.
func (r Result) Issue() model.Issue {
	return model.Issue{
		Linter:   Name,
		Rule:     r.Rule,
		Severity: model.SeverityWarn,
		Position: r.Position,
		Symbol:   r.Symbol,
		Message:  r.Message,
		Attrs:    map[string]string{"kind": r.Kind.String()},
	}
}

// Results is what the linter found, and is what the framework ranges over. The
// findings keep their own shape and are turned into issues one at a time,
// rather than the whole slice being copied into a common one.
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
