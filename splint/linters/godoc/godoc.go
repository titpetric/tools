// Package godoc reports exported symbols whose doc comment is missing, does
// not open on the symbol it documents, or runs long enough to be a smell.
package godoc

import (
	"context"
	"fmt"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// Name is how the linter is selected and how its issues are labelled.
const Name = "godoc"

// The rules this linter reports under.
const (
	RuleMissing = "missing"
	RuleFormat  = "format"
	RuleVerbose = "verbose"
)

// maxDocLines is how many lines a doc comment may run to before it reads as a
// symbol doing too much rather than a symbol well explained.
const maxDocLines = 10

// blockGap is how many lines apart two declarations may sit and still be read
// as one const or var block, which one comment above it documents.
const blockGap = 10

// Linter checks the godoc of every exported symbol.
type Linter struct{}

// New returns the linter.
func New() *Linter {
	return &Linter{}
}

// Name is the linter name.
func (l *Linter) Name() string {
	return Name
}

// Lint reports the godoc of every exported symbol outside main and the test
// packages. A command documents nothing to a consumer, and a test package is
// not a surface anyone reads.
func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	var results Results

	for _, def := range root.Packages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if def.Package.Package == "main" || def.Package.TestPackage {
			continue
		}

		var found []Result
		exported := 0
		for _, decls := range []model.DeclarationList{def.Types, def.Funcs, def.Consts, def.Vars} {
			exported += countExported(decls)
			found = append(found, check(def.Package, decls)...)
		}

		metric := results.count(def.Package, exported)
		metric.Documented = exported - documentedShort(found)
		for _, result := range found {
			results.add(metric, result)
		}
	}

	return results, nil
}

// countExported is how many symbols of a list a reader can reach, which is
// what the documentation is measured against.
func countExported(decls model.DeclarationList) int {
	count := 0
	for _, decl := range decls {
		if decl.IsExported() && !decl.IsTestScope() {
			count++
		}
	}
	return count
}

// documentedShort is how many symbols the findings are about, which is not the
// same as how many findings there are: one symbol can be reported once.
func documentedShort(found []Result) int {
	seen := map[string]bool{}
	for _, result := range found {
		seen[result.Position.Ref()+" "+result.Symbol] = true
	}
	return len(seen)
}

// check reports one kind of declaration in one package.
//
// Declarations are read in blocks: a run of exported names in the same file,
// none more than blockGap lines from the one above it, is one const or var
// block, and a comment on the first of them documents all of them.
func check(pkg model.Package, decls model.DeclarationList) []Result {
	var results []Result

	for _, block := range blocks(decls) {
		if len(block) == 1 {
			results = append(results, validate(pkg, block[0])...)
			continue
		}

		documented := strings.TrimSpace(block[0].Doc) != ""
		for _, decl := range block {
			switch {
			case strings.TrimSpace(decl.Doc) != "":
				results = append(results, validate(pkg, decl)...)
			case !documented:
				results = append(results, result(pkg, decl, RuleMissing, "exported symbol lacks a godoc comment"))
			}
		}
	}

	return results
}

// blocks groups the exported declarations that read as one block.
func blocks(decls model.DeclarationList) []model.DeclarationList {
	var (
		groups  []model.DeclarationList
		current model.DeclarationList
	)

	for _, decl := range decls {
		if !decl.IsExported() || decl.IsTestScope() {
			continue
		}
		if len(current) > 0 {
			last := current[len(current)-1]
			if last.File != decl.File || decl.Line-last.Line > blockGap {
				groups = append(groups, current)
				current = nil
			}
		}
		current = append(current, decl)
	}
	if len(current) > 0 {
		groups = append(groups, current)
	}

	return groups
}

// validate reports one documented or undocumented declaration.
func validate(pkg model.Package, decl *model.Declaration) []Result {
	doc := strings.TrimSpace(decl.Doc)
	if doc == "" {
		return []Result{result(pkg, decl, RuleMissing, "exported symbol lacks a godoc comment")}
	}

	// A block declaring more than one name is documented as a block, so the
	// comment names none of them and only the punctuation is checked.
	if len(decl.Names) > 1 {
		if !endsInPunctuation(doc) {
			return []Result{result(pkg, decl, RuleFormat, "godoc should end in punctuation")}
		}
		return nil
	}

	words := strings.Fields(doc)
	if len(words) == 0 {
		return nil
	}
	if !strings.EqualFold(words[0], decl.Name) {
		return []Result{result(pkg, decl, RuleFormat, fmt.Sprintf("godoc should open on %q and opens on %q", decl.Name, words[0]))}
	}
	if !endsInPunctuation(doc) {
		return []Result{result(pkg, decl, RuleFormat, "godoc should end in punctuation")}
	}
	if lines := strings.Count(doc, "\n") + 1; lines > maxDocLines {
		return []Result{result(pkg, decl, RuleVerbose, fmt.Sprintf("godoc runs to %d lines, which usually says the symbol does too much", lines))}
	}

	return nil
}

// endsInPunctuation reports whether a doc comment ends the way a sentence
// does. A closing backtick counts: a comment ending on a code span has said
// what it had to say.
func endsInPunctuation(doc string) bool {
	switch doc[len(doc)-1] {
	case '.', '!', '?', '`':
		return true
	}
	return false
}

// result builds one finding.
func result(pkg model.Package, decl *model.Declaration, rule, message string) Result {
	return Result{
		Rule:     rule,
		Symbol:   decl.Symbol(),
		Kind:     decl.Kind,
		Position: decl.Position(pkg),
		Message:  message,
	}
}
