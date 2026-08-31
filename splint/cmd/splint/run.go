package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/titpetric/tools/splint"
	"github.com/titpetric/tools/splint/analyzer"
	"github.com/titpetric/tools/splint/linters"
	"github.com/titpetric/tools/splint/loader"
	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/render"
	"github.com/titpetric/tools/splint/report"
	"github.com/titpetric/tools/splint/schema"
	"github.com/titpetric/tools/splint/simpleparser"
)

// Exit codes: nothing found, something found, and the run itself failing,
// which main reports as 2.
const (
	exitClean = 0
	exitFound = 1
)

// run parses the tree, lints the document and writes the report.
func run(ctx context.Context, args []string, w io.Writer) (int, error) {
	cfg, err := parseOptions(args)
	if err != nil {
		return 0, err
	}
	if cfg.help {
		printHelp(w)
		return exitClean, nil
	}

	selected, unknown := linters.Named(cfg.linters...)
	if len(unknown) > 0 {
		return 0, fmt.Errorf("no such linter: %s (have %s)", strings.Join(unknown, ", "), strings.Join(linters.Names(), ", "))
	}

	root, err := document(ctx, cfg)
	if err != nil {
		return 0, err
	}

	if cfg.output != "" {
		if err := loader.Save(cfg.output, root); err != nil {
			return 0, err
		}
	}

	if cfg.schema {
		return exitClean, schema.Write(w, root, schema.Options{StripPrefix: cfg.stripPrefix})
	}

	reports := make([]model.LintReport, 0, len(selected))
	for _, linter := range selected {
		one, err := linter.Lint(ctx, root)
		if err != nil {
			return 0, fmt.Errorf("%s: %w", linter.Name(), err)
		}
		reports = append(reports, one)
	}

	// The statistics are what the linters measured, which is a different
	// question from what they found: a run asking for one is not asking for
	// the other.
	if cfg.stats {
		return exitClean, render.Stats(w, reports, render.Format(cfg.format))
	}

	result := report.New(reports...)
	if err := render.Issues(w, result, render.Format(cfg.format)); err != nil {
		return 0, err
	}

	if result.Len() > 0 {
		return exitFound, nil
	}
	return exitClean, nil
}

// document is the tree as the options ask for it: read back from a file when
// one is named, and parsed when none is.
func document(ctx context.Context, cfg *config) (*model.DocumentRoot, error) {
	if cfg.input != "" {
		return loader.Load(cfg.input)
	}

	parser, err := parserFor(cfg)
	if err != nil {
		return nil, err
	}
	return parser.Parse(ctx)
}

// parserFor returns the parser the options name.
//
// The two are constructed identically and return the same document, so this is
// the whole of what selecting one costs.
func parserFor(cfg *config) (splint.Parser, error) {
	switch cfg.parser {
	case analyzer.ParserName:
		return analyzer.New(cfg.options), nil
	case simpleparser.ParserName:
		return simpleparser.New(cfg.options), nil
	}
	return nil, fmt.Errorf("no such parser: %s (have %s, %s)", cfg.parser, analyzer.ParserName, simpleparser.ParserName)
}
