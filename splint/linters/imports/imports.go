// Package imports reports what a file's import block says and what it should
// say: two packages reached under one name, a block that is not written to the
// house rule, an import nothing reaches, a name nothing can place, and a
// dependency that has spread across the tree.
//
// Everything it reports under a fixable rule is what splint fix writes. The
// rule and the rewrite are the same function of the same document, so a run
// that reports a file and a run that fixes it never disagree about what the
// block should hold.
package imports

import (
	"context"
	"fmt"
	"strings"

	"github.com/titpetric/tools/splint/importfmt"
	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/resolve"
)

// Name is how the linter is selected and how its issues are labelled.
const Name = "imports"

// The rules this linter reports under.
const (
	// RuleCollision is two files of one package reaching different modules
	// under the same short name.
	RuleCollision = "collision"

	// RuleFormat is an import block that is not what the house rule says.
	RuleFormat = "format"

	// RuleUnused is an import no name in the file reaches.
	RuleUnused = "unused"

	// RuleUnresolved is a name the file reaches that nothing can place.
	RuleUnresolved = "unresolved"

	// RulePollution is an external dependency imported from more of the tree
	// than one place.
	RulePollution = "pollution"
)

// FixAttr is the attribute a fixable issue carries, and its value is the
// command that clears it.
const FixAttr = "fix"

// FixCommand is what clears every fixable issue this linter reports.
const FixCommand = "splint fix"

// Pollution is when a dependency reaching many files is reported.
type Pollution struct {
	// PerPackage is how many files of one package may import one external
	// dependency before it is reported. Zero turns the rule off.
	PerPackage int

	// FileShare is the share of the files of the tree one external dependency
	// may reach before it is reported, between 0 and 1. Zero turns the rule
	// off.
	FileShare float64

	// Tests counts the test files.
	Tests bool
}

// Linter checks the imports of every package.
type Linter struct {
	// Options is the house rule the format rule is checked against, and the
	// one the fixer writes to.
	Options importfmt.Options

	// Aliases are the configured hints resolution consults before it consults
	// the tree.
	Aliases map[string]string

	// Pollution is when a dependency reaching many files is reported.
	Pollution Pollution
}

// New returns the linter under the default rule.
func New() *Linter {
	return &Linter{
		Options:   importfmt.NewOptions(),
		Pollution: Pollution{PerPackage: 2, FileShare: 0.5, Tests: true},
	}
}

// Name is the linter name.
func (l *Linter) Name() string {
	return Name
}

// Lint reports what every package's imports say.
func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	var results Results

	index := resolve.New(root, l.Aliases)
	options := l.options(root)

	for _, def := range root.Packages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		names, collisions := def.Imports.Map(def.Imports.All())
		metric := results.count(def, len(names))

		for _, collision := range collisions {
			results.add(metric, Result{
				Rule:     RuleCollision,
				Severity: model.SeverityWarn,
				Position: model.Position{Package: def.Package.Package, File: importFile(def)},
				Message:  collision.Error(),
			})
		}

		l.block(&results, metric, root, def, index, options)
	}

	l.spread(root, &results)

	return results, nil
}

// options is the house rule this run formats to, with the project group
// defaulting to the module the tree builds under.
func (l *Linter) options(root *model.DocumentRoot) importfmt.Options {
	options := l.Options
	if options.Project == "" {
		options.Project = importfmt.Project(root)
	}
	return options
}

// block reports what each file of a package should import against what it
// does.
func (l *Linter) block(results *Results, metric *Metric, root *model.DocumentRoot, def *model.Definition, index importfmt.Resolver, options importfmt.Options) {
	decisions := importfmt.Decide(root, def, index, options)

	for _, file := range def.Files {
		decision, known := decisions[file.Name]
		if !known {
			continue
		}
		metric.Considered++

		position := model.Position{Package: def.Package.Package, File: filePath(def, file.Name)}

		for _, unresolved := range decision.Unresolved {
			results.add(metric, Result{
				Rule:     RuleUnresolved,
				Severity: model.SeverityError,
				Position: position,
				Symbol:   unresolved.Name,
				Message:  unresolvedMessage(unresolved),
			})
			metric.Unresolved++
		}

		for _, spec := range decision.Removed {
			results.add(metric, Result{
				Rule:     RuleUnused,
				Severity: model.SeverityError,
				Position: model.Position{Package: position.Package, File: position.File, Line: spec.Line},
				Symbol:   spec.Ref(),
				Message:  fmt.Sprintf("%s is imported and no name in the file reaches it", spec.Path),
				Fixable:  decision.Sound(),
			})
			metric.Unused++
		}

		if !decision.Changed(file.ImportDecls) {
			metric.Formatted++
			continue
		}

		results.add(metric, Result{
			Rule:     RuleFormat,
			Severity: model.SeverityError,
			Position: model.Position{Package: position.Package, File: position.File, Line: firstLine(file.ImportDecls)},
			Message:  formatMessage(file.ImportDecls, decision),
			Fixable:  decision.Sound(),
		})
	}
}

// unresolvedMessage says what could not be placed and why.
func unresolvedMessage(unresolved importfmt.Unresolved) string {
	reached := unresolved.Name
	if len(unresolved.Symbols) > 0 {
		reached += "." + unresolved.Symbols[0]
	}

	if len(unresolved.Candidates) > 0 {
		return fmt.Sprintf("%s reaches %s and %d packages answer to it: %s",
			reached, unresolved.Name, len(unresolved.Candidates), strings.Join(unresolved.Candidates, ", "))
	}

	return fmt.Sprintf("%s reaches %s and no package of the tree, no file of it and no requirement is called that",
		reached, unresolved.Name)
}

// formatMessage says what is wrong with the block rather than printing the one
// it should be, which is a dozen lines a report has no room for.
func formatMessage(current model.ImportDeclList, decision importfmt.Decision) string {
	var reasons []string

	if len(current) > 1 {
		reasons = append(reasons, fmt.Sprintf("%d import declarations to merge", len(current)))
	}
	if len(decision.Added) > 0 {
		reasons = append(reasons, fmt.Sprintf("%d to add", len(decision.Added)))
	}
	if len(decision.Removed) > 0 {
		reasons = append(reasons, fmt.Sprintf("%d to remove", len(decision.Removed)))
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "the imports are not grouped and ordered to the house rule")
	}

	if !decision.Sound() {
		return "the import block cannot be written: " + strings.Join(reasons, ", ") + ", and a name in the file does not resolve"
	}

	return "the import block is not what it should be: " + strings.Join(reasons, ", ")
}

// firstLine is where a file's imports begin, and is zero for a file that has
// none: an issue about an import block a file does not have belongs to the
// file rather than to a line of it.
func firstLine(decls model.ImportDeclList) int {
	if len(decls) == 0 {
		return 0
	}
	return decls[0].Line
}

// filePath is a file as a reader opens it, which is the package directory and
// the name.
func filePath(def *model.Definition, name string) string {
	dir := strings.TrimPrefix(strings.TrimPrefix(def.Package.Path, "."), "/")
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

// importFile names the package a collision is in. An import set is keyed on
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
