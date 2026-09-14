package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/titpetric/tools/splint/commands/coverage"
	"github.com/titpetric/tools/splint/commands/diff"
	"github.com/titpetric/tools/splint/commands/docs"
	"github.com/titpetric/tools/splint/commands/sizes"
	"github.com/titpetric/tools/splint/coverprofile"
	"github.com/titpetric/tools/splint/linters"
	"github.com/titpetric/tools/splint/linters/modcheck"
	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/model/loader"
	"github.com/titpetric/tools/splint/parsers/analyzer"
	"github.com/titpetric/tools/splint/parsers/simpleparser"
	"github.com/titpetric/tools/splint/pkg/splint"
	"github.com/titpetric/tools/splint/render"
	"github.com/titpetric/tools/splint/report"
	"github.com/titpetric/tools/splint/schema"
)

// Exit codes: nothing found, something found, and the run itself failing,
// which main reports as 2.
const (
	exitClean = 0
	exitFound = 1
)

// run parses the tree, lints the document and writes the report.
//
// The report goes to w and anything said along the way goes to progress. The
// two are one stream for splint fix, whose whole output is what it rewrote,
// and two for splint --fix, where w is carrying a report a program parses: a
// line of progress written into it is a line of invalid JSON.
func run(ctx context.Context, args []string, w, progress io.Writer) (int, error) {
	cfg, err := parseOptions(args)
	if err != nil {
		return 0, err
	}
	if cfg.help {
		return exitClean, writeHelp(w, helpSpec(cfg))
	}

	// A fix is a rewrite of the tree, and a report of a tree that has just
	// been rewritten has to be a report of what the rewrite left. Both paths
	// therefore fix first and parse after.
	if cfg.command == commandFix {
		if _, err := runFix(ctx, cfg, w); err != nil {
			return 0, err
		}
		return exitClean, nil
	}

	// A diff is between two documents already written; no tree is parsed.
	// The JSON is one line, which is what worktree reads.
	if cfg.command == commandDiff {
		result, err := diff.Load(cfg.oldFile, cfg.newFile, cfg.diffOptions)
		if err != nil {
			return 0, err
		}
		if cfg.data() {
			encoded, err := json.Marshal(result)
			if err != nil {
				return 0, err
			}
			_, err = fmt.Fprintln(w, string(encoded))
			return exitClean, err
		}
		return exitClean, diff.Write(w, result, cfg.options.Verbose)
	}

	selected, unknown := linters.Named(cfg.linters...)
	if len(unknown) > 0 {
		return 0, fmt.Errorf("no such linter: %s (have %s)", strings.Join(unknown, ", "), strings.Join(linters.Names(), ", "))
	}

	if cfg.offline {
		offline(selected)
	}

	if cfg.fix {
		if _, err := runFix(ctx, cfg, progress); err != nil {
			return 0, err
		}
	}

	root, err := document(ctx, cfg)
	if err != nil {
		return 0, err
	}

	if err := configure(selected, cfg, root); err != nil {
		return 0, err
	}

	if cfg.coverageProfile != "" {
		if err := coverprofile.Apply(root, cfg.coverageProfile); err != nil {
			return 0, err
		}
	}

	if cfg.output != "" {
		if err := loader.Save(cfg.output, written(root, cfg)); err != nil {
			return 0, err
		}
	}

	// docs and coverage render the document rather than lint it. The docs get
	// the parse whole, because the examples they print live in the test
	// packages; the coverage report gets what a written document would hold,
	// so a run over a parse reports what a run over --input would.
	if cfg.command == commandDocs {
		return exitClean, docs.Write(w, root, docs.Options{
			Format:      cfg.render,
			Split:       cfg.split,
			OutDir:      cfg.out,
			StripPrefix: cfg.stripPrefix,
			Model:       cfg.modelMode,
			Hide:        cfg.hide,
			Root:        cfg.docsRoot,
			Title:       cfg.docsTitle,
			Symbol:      cfg.docsSymbol,
			Verbose:     cfg.options.Verbose,
		})
	}

	if cfg.command == commandSizes {
		if cfg.render == "d2" {
			return exitClean, sizes.WriteD2(w, root.Packages)
		}
		return exitClean, sizes.Write(w, root.Packages)
	}

	if cfg.command == commandCoverage {
		reported := written(root, cfg)
		if cfg.data() {
			return exitClean, writeData(w, cfg, coverage.Data(reported, cfg.options.Verbose))
		}
		return exitClean, coverage.Write(w, reported, coverage.Options{
			Template: cfg.template,
			Verbose:  cfg.options.Verbose,
		})
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
		if cfg.data() {
			return exitClean, writeData(w, cfg, measured(reports))
		}
		return exitClean, render.Stats(w, reports)
	}

	result := report.New(reports...)
	if cfg.data() {
		if err := writeData(w, cfg, result); err != nil {
			return 0, err
		}
	} else if err := render.Issues(w, result); err != nil {
		return 0, err
	}

	if result.Len() > 0 {
		return exitFound, nil
	}
	return exitClean, nil
}

// written is the document as a file holds it. The linters read every test file
// of the tree; a reader of the document gets them only when --include-tests
// asked for them.
func written(root *model.DocumentRoot, cfg *config) *model.DocumentRoot {
	if cfg.includeTests {
		return root
	}
	return root.WithoutTests()
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

// offline takes the linters off the network. modcheck is the one that reaches
// it, and what it asks about a module is a size, which the cache holds from
// the runs that did ask.
func offline(selected []model.Linter) {
	for _, linter := range selected {
		module, ok := linter.(*modcheck.Linter)
		if !ok || module.Proxy == nil {
			continue
		}
		module.Proxy.Offline = true
	}
}

// measurement is what one linter measured, which is the table it would have
// drawn and the numbers behind it.
type measurement struct {
	Linter     string             `json:"Linter" yaml:"Linter"`
	Metrics    model.LintMetrics  `json:"Metrics,omitzero" yaml:"Metrics,omitempty"`
	Statistics []model.Statistics `json:"Statistics,omitempty" yaml:"Statistics,omitempty"`
}

// measured is what every linter of a run measured.
func measured(reports []model.LintReport) []measurement {
	out := make([]measurement, 0, len(reports))

	for _, one := range reports {
		if one == nil {
			continue
		}
		out = append(out, measurement{
			Linter:     one.Linter(),
			Metrics:    one.Metrics(),
			Statistics: one.Statistics(),
		})
	}

	return out
}

// writeData writes what a rendering would have drawn, for a reader that is a
// program.
//
// One data model answers both: every field carries a json and a yaml tag
// naming the same key, and a severity is a text marshaller, which both
// encoders read. The JSON is indented, because a document a person opens is
// one a person reads.
func writeData(w io.Writer, cfg *config, value any) error {
	if cfg.yaml {
		encoder := yaml.NewEncoder(w)
		defer encoder.Close()
		return encoder.Encode(value)
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

// data reports a run asked for the findings rather than a rendering of them.
func (c *config) data() bool {
	return c.json || c.yaml
}
