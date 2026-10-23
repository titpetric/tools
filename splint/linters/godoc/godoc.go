// Package godoc reports exported symbols whose doc comment is missing, does
// not open on the symbol it documents, or runs long enough to be a smell.
package godoc

import (
	"context"
	"fmt"
	"path"
	"strings"

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
)

// maxDocLines is how many lines a doc comment may run to before it reads as a
// symbol doing too much, where a short comment reads as a symbol well explained.
const maxDocLines = 10

// blockGap is how many lines apart two declarations may sit and still read
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

		generated := generatedFiles(def)

		var found []Result
		exported := 0
		for _, decls := range []model.DeclarationList{def.Types, def.Funcs, def.Consts, def.Vars} {
			exported += countExported(decls, generated)
			found = append(found, check(def.Package, decls, generated)...)
		}

		metric := results.count(def.Package, exported)
		metric.Documented = exported - documentedShort(found)
		for _, result := range found {
			results.add(metric, result)
		}
	}

	return results, nil
}

// generatedFiles is the set of files in def written by a generator, whose
// comments are the generator's to write.
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

// countExported is how many symbols of a list a reader can reach, which is
// what the linter measures the documentation against.
func countExported(decls model.DeclarationList, generated map[string]bool) int {
	count := 0
	for _, decl := range decls {
		if decl.IsExported() && !decl.IsTestScope() && !generated[decl.File] {
			count++
		}
	}
	return count
}

// documentedShort is how many symbols the findings are about, which is not the
// same as how many findings there are: the linter reports one symbol once.
func documentedShort(found []Result) int {
	seen := map[string]bool{}
	for _, result := range found {
		seen[result.Position.Ref()+" "+result.Symbol] = true
	}
	return len(seen)
}

// check reports one kind of declaration in one package.
//
// The check reads declarations in blocks: a run of exported names in the same file,
// none more than blockGap lines from the one above it, is one const or var
// block, and a comment on the first of them documents each one.
func check(pkg model.Package, decls model.DeclarationList, generated map[string]bool) []Result {
	var results []Result

	for _, block := range blocks(decls, generated) {
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
//
// The check leaves out a generated file. Whatever wrote the file writes its
// comment, so any finding there is a finding nobody in this tree can act on: the
// comment has to come from the generator or not at all.
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
func validate(pkg model.Package, decl *model.Declaration) []Result {
	doc := strings.TrimSpace(decl.Doc)
	if doc == "" {
		return []Result{result(pkg, decl, RuleMissing, "exported symbol lacks a godoc comment")}
	}

	// A block declaring more than one name carries a block comment, so the
	// comment names none of them and the check looks at the punctuation alone.
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
	if text, exits := len(prose(doc)), exitPoints(decl); exits > 0 && text > exits {
		return []Result{result(pkg, decl, RuleOutweighs, fmt.Sprintf("godoc runs to %d lines over %s; keep the function's comment to its exits, put intent on the type or in docs.go", text, plural(exits, "exit point", "exit points")))}
	}

	return nil
}

// prose is the lines of a comment that carry text. A paragraph break carries
// none, and an indented block is an example, which the generated reference
// prints and which weighs nothing against the exits.
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

// exitPoints is how many prose lines a function's comment may run to, one per
// exit and at least one. Zero exempts a declaration with no body, and doc.go
// and docs.go, where a package keeps its long prose.
func exitPoints(decl *model.Declaration) int {
	if decl.Complexity == nil || decl.Complexity.Lines == 0 {
		return 0
	}
	switch path.Base(decl.File) {
	case "doc.go", "docs.go":
		return 0
	}
	return max(decl.Complexity.Exits, 1)
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
