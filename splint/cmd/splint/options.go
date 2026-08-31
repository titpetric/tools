package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/titpetric/tools/splint"
	"github.com/titpetric/tools/splint/analyzer"
	"github.com/titpetric/tools/splint/linters"
	"github.com/titpetric/tools/splint/simpleparser"
)

// config is what one run was asked for.
type config struct {
	// options are what the parser is given.
	options splint.Options

	// parser names which parser reads the tree. The ast parser is the default
	// and stays the default: it is the exact reading, and the quick one is
	// something a caller asks for on purpose.
	parser string

	// input reads a document back from a file instead of parsing, and output
	// writes the parsed one to a file as well as linting it.
	input  string
	output string

	// schema writes the document as a JSON Schema instead of linting it, and
	// stats writes what the linters measured instead of what they found.
	schema bool
	stats  bool

	// stripPrefix are package prefixes to take off a schema definition name.
	stripPrefix []string

	// linters selects the linters by name, and is every linter when empty.
	linters []string

	// format names the rendering, and is chosen by the destination when empty.
	format string

	help bool
}

// parseOptions reads the command line.
//
// The trailing argument is the pattern, so "splint ./..." reads the whole tree
// and "splint ." reads one package, which is how every other tool here spells
// it.
func parseOptions(args []string) (*config, error) {
	cfg := &config{options: splint.NewOptions(), parser: analyzer.ParserName}

	var selected, strip string
	fs := flag.NewFlagSet("splint", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.options.SourcePath, "i", cfg.options.SourcePath, "source path to read")
	fs.StringVar(&cfg.parser, "parser", cfg.parser, "parser to read the tree with")
	fs.StringVar(&cfg.input, "input", "", "read a document from a file instead of parsing")
	fs.StringVar(&cfg.output, "output", "", "write the parsed document to a file")
	fs.BoolVar(&cfg.schema, "schema", false, "write the document as a JSON Schema instead of linting it")
	fs.BoolVar(&cfg.stats, "stats", false, "write what the linters measured instead of what they found")
	fs.StringVar(&strip, "strip-prefix", "", "package prefixes to strip from schema names, comma separated")
	fs.StringVar(&selected, "linters", "", "linters to run, comma separated ("+strings.Join(linters.Names(), ", ")+")")
	fs.StringVar(&cfg.format, "format", "auto", "output format: auto, markdown, terminal or github")
	fs.BoolVar(&cfg.options.IncludeTests, "include-tests", false, "read the test files too")
	fs.BoolVar(&cfg.options.IncludeSources, "include-sources", false, "keep the source of every declaration")
	fs.BoolVar(&cfg.options.Verbose, "v", false, "say what is being read")
	fs.BoolVar(&cfg.help, "help", false, "print this help")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			cfg.help = true
			return cfg, nil
		}
		return nil, err
	}

	if rest := fs.Args(); len(rest) > 0 {
		cfg.options.Pattern = rest[len(rest)-1]
	}
	cfg.linters = commaList(selected)
	cfg.stripPrefix = commaList(strip)

	// A schema is written from the types of a tree, and a type is described by
	// what it declares, so the sources come along whether or not they were
	// asked for.
	if cfg.schema {
		cfg.options.IncludeSources = true
	}

	// A check that pairs a file with its test, or looks for the test of a
	// symbol, has nothing to read unless the tests are read too.
	cfg.options.IncludeTests = true

	return cfg, nil
}

// commaList splits a comma separated flag into its entries.
func commaList(value string) []string {
	var out []string

	for _, entry := range strings.Split(value, ",") {
		if entry = strings.TrimSpace(entry); entry != "" {
			out = append(out, entry)
		}
	}

	return out
}

// printHelp writes what the command takes.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, `Usage: splint [options] [pattern]

Lints a Go tree through the splint model. The pattern is "." for one package
and "./..." for everything below the source path.

Options:
  -i PATH             source path to read (default ".")
  -parser NAME        %s or %s (default %s)
  -input FILE         read a document from a .json or .yml file instead
  -output FILE        write the parsed document to a .json or .yml file
  -schema             write the document as a JSON Schema instead of linting
  -stats              write what the linters measured, one table each
  -strip-prefix LIST  package prefixes to strip from schema names
  -linters LIST       comma separated: %s
  -format NAME        auto, markdown, terminal or github (default auto)
  -include-tests      read the test files too
  -include-sources    keep the source of every declaration
  -v                  say what is being read

The ast parser is the default. The simple parser reads the source without
building a syntax tree: it produces the same document, reads source that does
not compile, and is an order of magnitude quicker.

Exits 1 when a linter found something, 2 when the run itself failed.
`, analyzer.ParserName, simpleparser.ParserName, analyzer.ParserName, strings.Join(linters.Names(), ", "))
}
