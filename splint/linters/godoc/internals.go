package godoc

import (
	"fmt"
	"path"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// internalsBudget is the one line an unexported symbol gets, for a rule its signature cannot state.
const internalsBudget = 1

// A generated file and doc.go are exempt, the same two the exported rules exempt.
func internals(pkg model.Package, decls model.DeclarationList, generated map[string]bool) []Result {
	var results []Result

	for _, decl := range decls {
		if decl.IsExported() || decl.IsTestScope() || generated[decl.File] {
			continue
		}
		switch path.Base(decl.File) {
		case "doc.go", "docs.go":
			continue
		}
		lines := len(prose(strings.TrimSpace(decl.Doc)))
		if lines <= internalsBudget {
			continue
		}
		results = append(results, result(pkg, decl, RuleInternals, internalsMessage(lines)))
	}

	return results
}

func internalsMessage(lines int) string {
	return fmt.Sprintf("godoc runs to %d prose lines on an unexported symbol, which godoc prints to nobody; keep the one line a rule from outside the file needs and delete the rest", lines)
}
