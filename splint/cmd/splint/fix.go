package main

import (
	"context"
	"fmt"
	"io"

	settings "github.com/titpetric/tools/splint/config"
	"github.com/titpetric/tools/splint/fix"
	"github.com/titpetric/tools/splint/importfmt"
	"github.com/titpetric/tools/splint/linters/imports"
	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/simpleparser"
)

// runFix rewrites the import block of every file that does not hold the one
// the rules describe, and says what it did.
//
// It parses the tree itself rather than taking the document the report is
// written from. The rewrite changes the tree, so a document read before it is
// a document of a tree that no longer exists, and a report written off one
// would name lines that have moved.
func runFix(ctx context.Context, cfg *config, w io.Writer) (int, error) {
	root, err := document(ctx, forFixing(cfg))
	if err != nil {
		return 0, err
	}

	plan, err := planFor(root, cfg)
	if err != nil {
		return 0, err
	}

	changed, err := fix.Apply(plan)
	if err != nil {
		return 0, err
	}

	// The count is bookkeeping and the rewrite is the work. A tree whose
	// configuration cannot be written has still been formatted, and failing
	// the run here would say it had not.
	if err := settings.AddFixed(cfg.options.SourcePath, len(changed)); err != nil {
		fmt.Fprintf(w, "the rewrite is done and the count is not recorded: %v\n", err)
	}

	return len(changed), writeFixed(w, plan, changed)
}

// forFixing is the run as the fixer reads the tree, which is with the quick
// parser unless the command line named one.
//
// The ast parser resolves a tree through the toolchain, and a file that is
// missing an import it needs does not compile: the tree the fixer is pointed
// at is the tree least likely to type check. The quick parser reads the text,
// so what is written is what it reports. It is also an order of magnitude
// faster over the same tree, and a formatter is run on every save.
func forFixing(cfg *config) *config {
	if cfg.parserNamed {
		return cfg
	}

	quick := *cfg
	quick.parser = simpleparser.ParserName
	return &quick
}

// planFor is what the fixer would write, under the rules the tree states.
func planFor(root *model.DocumentRoot, cfg *config) (*fix.Plan, error) {
	rules, err := settings.Load(cfg.options.SourcePath)
	if err != nil {
		return nil, err
	}

	options := rules.ImportOptions(importfmt.Project(root))
	return fix.Build(root, options, rules.Imports.Aliases), nil
}

// writeFixed says which files were rewritten and which were left alone.
func writeFixed(w io.Writer, plan *fix.Plan, changed []string) error {
	for _, name := range changed {
		if _, err := fmt.Fprintln(w, "fixed:", name); err != nil {
			return err
		}
	}

	for _, name := range plan.Skipped {
		if _, err := fmt.Fprintln(w, "left alone:", name, "- a name in it does not resolve"); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintf(w, "%s rewritten, %d left alone.\n", count(len(changed), "file"), len(plan.Skipped))
	return err
}

// count writes a number and the thing it counts, plural where it should be.
func count(n int, thing string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, thing)
	}
	return fmt.Sprintf("%d %ss", n, thing)
}

// configure sets the import linter to the rules the tree states, so a report
// judges a file against the same block the fixer would write into it.
//
// The linter is the one that takes settings and the registry builds it with
// none, which is how --offline reaches modcheck too.
func configure(selected []model.Linter, cfg *config, root *model.DocumentRoot) error {
	rules, err := settings.Load(cfg.options.SourcePath)
	if err != nil {
		return err
	}

	for _, linter := range selected {
		rule, ok := linter.(*imports.Linter)
		if !ok {
			continue
		}
		rule.Options = rules.ImportOptions(importfmt.Project(root))
		rule.Aliases = rules.Imports.Aliases
		rule.Pollution = imports.Pollution{
			PerPackage: rules.Imports.Pollution.PerPackage,
			FileShare:  rules.Imports.Pollution.FileShare,
			Tests:      rules.Imports.Pollution.CountTests(),
		}
	}

	return nil
}
