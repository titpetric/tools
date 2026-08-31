// Package funcreturns reports functions returning an error or a bool before
// the value they qualify, which is the reverse of how every caller reads them.
package funcreturns

import (
	"context"
	"fmt"

	"github.com/titpetric/tools/splint/model"
)

// Name is how the linter is selected and how its issues are labelled.
const Name = "func-returns"

// RuleOrder is the one rule this linter reports under.
const RuleOrder = "order"

// Linter checks the return order of every function.
type Linter struct{}

// New returns the linter.
func New() *Linter {
	return &Linter{}
}

// Name is the linter name.
func (l *Linter) Name() string {
	return Name
}

// Lint reports every function returning an error or a bool before the value it
// qualifies. A function returning one value or none has no order to get wrong.
func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	var results Results

	for _, def := range root.Packages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, decl := range def.Funcs {
			if decl.IsTestScope() || len(decl.Returns) < 2 {
				continue
			}
			expected := expectedOrder(decl.Returns)
			if sameOrder(decl.Returns, expected) {
				continue
			}
			results = append(results, Result{
				Rule:     RuleOrder,
				Symbol:   decl.Symbol(),
				Returns:  decl.Returns,
				Expected: expected,
				Position: decl.Position(def.Package),
				Message:  fmt.Sprintf("returns read %v and should read %v", decl.Returns, expected),
			})
		}
	}

	return results, nil
}

// expectedOrder is the returns as they ought to read: the values first, then
// the bool that says whether they are there, then the error that says why they
// are not. A caller reads them in that order whatever the order they come in.
func expectedOrder(returns []string) []string {
	var normal []string
	var bools []string
	var errs []string

	for _, ret := range returns {
		switch ret {
		case "error":
			errs = append(errs, ret)
		case "bool":
			bools = append(bools, ret)
		default:
			normal = append(normal, ret)
		}
	}

	result := make([]string, 0, len(returns))
	result = append(result, normal...)
	result = append(result, bools...)
	result = append(result, errs...)
	return result
}

// sameOrder reports whether the returns are already in the expected order.
func sameOrder(returns, expected []string) bool {
	if len(returns) != len(expected) {
		return false
	}
	for i := range returns {
		if returns[i] != expected[i] {
			return false
		}
	}
	return true
}
