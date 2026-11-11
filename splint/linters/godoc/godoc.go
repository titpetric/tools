// Package godoc reports exported symbols whose doc comment is missing, does
// not open on the symbol it documents, runs long enough to be a smell, or
// says only what the signature below it already says.
package godoc

import (
	"context"
	"fmt"
	"path"
	"strings"
	"unicode"

	"github.com/titpetric/tools/splint/model"
)

// Name selects the linter and labels its issues.
const Name = "godoc"

// The rules this linter reports under.
const (
	RuleMissing   = "missing"
	RuleFormat    = "format"
	RuleVerbose   = "verbose"
	RuleOutweighs = "outweighs"
	RuleRestates  = "restates"
	RuleInternals = "internals"
)

const maxDocLines = 10

// Two declarations further apart than this are two blocks, documented separately.
const blockGap = 10

// Linter checks the godoc of every exported symbol outside main and the test
// packages: a command documents nothing to a consumer. A test package is
// no surface anyone reads.
type Linter struct {
	// Brevity caps every doc comment at one prose line.
	Brevity bool
}

// New returns the linter with the per-block budget, which Brevity narrows to one.
func New() *Linter {
	return &Linter{}
}

// Name selects the linter on a --linters list and labels its issues.
func (l *Linter) Name() string {
	return Name
}

// Lint reports the godoc of every exported symbol it reads.
func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	var results Results

	for _, def := range root.Packages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if def.Package.Package == "main" || def.Package.TestPackage {
			continue
		}

		generated := generatedFiles(def)

		var found, hidden []Result
		exported := 0
		for _, decls := range []model.DeclarationList{def.Types, def.Funcs, def.Consts, def.Vars} {
			exported += countExported(decls, generated)
			found = append(found, check(def.Package, decls, generated, l.Brevity)...)
			hidden = append(hidden, internals(def.Package, decls, generated)...)
		}

		metric := results.count(def.Package, exported)
		metric.Documented = exported - documentedShort(found)
		for _, result := range append(found, hidden...) {
			results.add(metric, result)
		}
	}

	return results, nil
}

func generatedFiles(def *model.Definition) map[string]bool {
	var generated map[string]bool
	for _, file := range def.Files {
		if !file.Generated {
			continue
		}
		if generated == nil {
			generated = map[string]bool{}
		}
		generated[file.Name] = true
	}
	return generated
}

func countExported(decls model.DeclarationList, generated map[string]bool) int {
	count := 0
	for _, decl := range decls {
		if decl.IsExported() && !decl.IsTestScope() && !generated[decl.File] {
			count++
		}
	}
	return count
}

// Counts a symbol once however many findings it drew.
func documentedShort(found []Result) int {
	seen := map[string]bool{}
	for _, result := range found {
		seen[result.Position.Ref()+" "+result.Symbol] = true
	}
	return len(seen)
}

// A comment on the first name of a const or var block documents every name in it.
func check(pkg model.Package, decls model.DeclarationList, generated map[string]bool, brevity bool) []Result {
	var results []Result

	for _, block := range blocks(decls, generated) {
		if len(block) == 1 {
			results = append(results, validate(pkg, block[0], brevity)...)
			continue
		}

		documented := strings.TrimSpace(block[0].Doc) != ""
		for _, decl := range block {
			switch {
			case strings.TrimSpace(decl.Doc) != "":
				results = append(results, validate(pkg, decl, brevity)...)
			case !documented:
				results = append(results, result(pkg, decl, RuleMissing, "exported symbol lacks a godoc comment"))
			}
		}
	}

	return results
}

// A generated file is skipped: its comment comes from the generator or not at all.
func blocks(decls model.DeclarationList, generated map[string]bool) []model.DeclarationList {
	var (
		groups  []model.DeclarationList
		current model.DeclarationList
	)

	for _, decl := range decls {
		if !decl.IsExported() || decl.IsTestScope() || generated[decl.File] {
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
func validate(pkg model.Package, decl *model.Declaration, brevity bool) []Result {
	doc := strings.TrimSpace(decl.Doc)
	if doc == "" {
		return []Result{result(pkg, decl, RuleMissing, "exported symbol lacks a godoc comment")}
	}

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
	lines := strings.Count(doc, "\n") + 1
	if lines > maxDocLines {
		return []Result{result(pkg, decl, RuleVerbose, fmt.Sprintf("godoc runs to %d lines, which usually says the symbol does too much", lines))}
	}
	if text, budget := len(prose(doc)), docBudget(decl, brevity); budget > 0 && text > budget {
		return []Result{result(pkg, decl, RuleOutweighs, outweighsMessage(text, budget, brevity))}
	}
	if Restates(decl, doc) {
		return []Result{result(pkg, decl, RuleRestates, restateMessage(decl))}
	}

	return nil
}

// outweighsMessage says where the prose over the budget goes.
func outweighsMessage(text, budget int, brevity bool) string {
	if brevity {
		return fmt.Sprintf("godoc runs to %d prose lines against a budget of one; say what it returns and what it refuses, and move the rest to docs/", text)
	}
	return fmt.Sprintf("godoc runs to %d prose lines against a budget of %s, one for the function and one per block it opens; cut it, or put the intent on the type or in doc.go", text, plural(budget, "line", "lines"))
}

// RuleMissing requires a comment on an exported symbol, so only an internal one may be deleted.
func restateMessage(decl *model.Declaration) string {
	if exported(decl.Name) {
		return "godoc says only what the signature says; a consumer reads this, so say what the signature cannot: what it refuses, what it costs, or why it exists"
	}
	return "godoc says only what the signature says; delete it"
}

// exported reports whether a name is one a consumer can reach.
func exported(name string) bool {
	if name == "" {
		return false
	}
	return unicode.IsUpper(rune(name[0]))
}

// A blank line and an indented example weigh nothing: the reference prints the example as code.
func prose(doc string) []string {
	var lines []string
	for _, line := range strings.Split(doc, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "    ") {
			continue
		}
		lines = append(lines, strings.TrimRight(line, " \t"))
	}
	return lines
}

// Zero exempts a declaration with no body, and doc.go, where long prose belongs.
func docBudget(decl *model.Declaration, brevity bool) int {
	if decl.Complexity == nil || decl.Complexity.Lines == 0 {
		return 0
	}
	switch path.Base(decl.File) {
	case "doc.go", "docs.go":
		return 0
	}
	if brevity {
		return 1
	}
	return 1 + decl.Complexity.Blocks
}

// A closing backtick counts: a comment ending on a code span has said what it had to say.
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
