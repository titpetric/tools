package main

import (
	"context"
	"fmt"
	"io"
	"slices"

	"github.com/titpetric/tools/splint/commands/fix"
	settings "github.com/titpetric/tools/splint/config"
	"github.com/titpetric/tools/splint/importfmt"
	"github.com/titpetric/tools/splint/linters/coverage"
	"github.com/titpetric/tools/splint/linters/filecheck"
	"github.com/titpetric/tools/splint/linters/imports"
	"github.com/titpetric/tools/splint/linters/integration"
	"github.com/titpetric/tools/splint/linters/pairing"
	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/parsers/simpleparser"
)

// runFix rewrites the import block of every file that does not hold the one
// the rules describe, and says what it did.
//
// It parses the tree itself and ignores the document the report reads. The
// rewrite changes the tree, so a document read before the rewrite describes a
// tree that has since changed, and a report built on one would name lines that
// have moved.
func runFix(ctx context.Context, cfg *config, w io.Writer) (int, error) {
	root, err := document(ctx, forFixing(cfg))
	if err != nil {
		return 0, err
	}

	plan, err := planFor(root, cfg)
	if err != nil {
		return 0, err
	}

	rewritten, formatted, err := fix.Apply(plan)
	if err != nil {
		return 0, err
	}

	// The count is bookkeeping and the rewrite is the work. A machine whose
	// counter refuses the write has still had its tree formatted, and failing
	// the run here would say it had not.
	//
	// It counts import rewrites alone, because the counter carries that name:
	// a file this run only formatted had the block the rules describe
	// before the run started.
	if err := settings.AddFixed(len(rewritten)); err != nil {
		fmt.Fprintf(w, "the rewrite is done and the count is not recorded: %v\n", err)
	}

	return len(rewritten) + len(formatted), writeFixed(w, plan, rewritten, formatted)
}

// forFixing is the run as the fixer reads the tree, which is with the quick
// parser unless the command line named one.
//
// The ast parser resolves a tree through the toolchain, and a file that is
// missing an import it needs does not compile: the tree the fixer points at
// is the tree least likely to type check. The quick parser reads the text,
// so it reports what the file holds. The quick parser also runs ten times
// faster over the same tree, and an editor runs a formatter on every save.
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

// writeFixed says which files had their import block rewritten, which had only
// formatting, and which the fixer left alone.
func writeFixed(w io.Writer, plan *fix.Plan, rewritten, formatted []string) error {
	for _, name := range rewritten {
		if _, err := fmt.Fprintln(w, "fixed:", name); err != nil {
			return err
		}
	}

	for _, name := range formatted {
		if _, err := fmt.Fprintln(w, "formatted:", name); err != nil {
			return err
		}
	}

	for _, skip := range plan.Skipped {
		if len(skip.Unresolved) == 0 {
			if _, err := fmt.Fprintln(w, "left alone:", skip.Name, "- a name in it does not resolve"); err != nil {
				return err
			}
			continue
		}
		for _, unresolved := range skip.Unresolved {
			if _, err := fmt.Fprintf(w, "left alone: %s - %s\n", skip.Name, unresolved.Message()); err != nil {
				return err
			}
		}
	}

	_, err := fmt.Fprintf(w, "%s rewritten, %s formatted, %d left alone.\n",
		count(len(rewritten), "file"), count(len(formatted), "file"), len(plan.Skipped))
	return err
}

// count writes a number and the thing it counts, plural for any count other than one.
func count(n int, thing string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, thing)
	}
	return fmt.Sprintf("%d %ss", n, thing)
}

// configure sets the linters that take settings to the rules the tree states,
// so a report judges a file against the same import block the fixer would
// write into it and against the length the tree allowed it.
//
// The registry builds every linter with its own defaults and this overwrites
// the ones the file speaks about; --offline reaches modcheck this way too.
func configure(selected []model.Linter, cfg *config, root *model.DocumentRoot) error {
	rules, err := settings.Load(cfg.options.SourcePath)
	if err != nil {
		return err
	}

	for _, linter := range selected {
		switch rule := linter.(type) {
		case *imports.Linter:
			rule.Options = rules.ImportOptions(importfmt.Project(root))
			rule.Aliases = rules.Imports.Aliases
			rule.Pollution = imports.Pollution{
				PerPackage: rules.Imports.Pollution.PerPackage,
				FileShare:  rules.Imports.Pollution.FileShare,
				Tests:      rules.Imports.Pollution.CountTests(),
				Ignore:     rules.Imports.Pollution.Ignore,
			}
		case *filecheck.Linter:
			rule.MaxLines = rules.Filecheck.MaxLines
			rule.Allow = rules.FilecheckAllowances()
		case *pairing.Linter:
			if rules.Integration.Tag != "" {
				rule.Tag = rules.Integration.Tag
			}
		case *coverage.Linter:
			if rules.Integration.Tag != "" {
				rule.Tag = rules.Integration.Tag
			}
		case *integration.Linter:
			// A run naming the linter runs it, with any tree setting.
			rule.Enabled = rules.Integration.Enabled || slices.Contains(cfg.linters, integration.Name)
			if rules.Integration.Tag != "" {
				rule.Tag = rules.Integration.Tag
			}
			rule.Storage = rules.Integration.Storage
			if rules.Integration.Depth > 0 {
				rule.Depth = rules.Integration.Depth
			}
		}
	}

	return nil
}
