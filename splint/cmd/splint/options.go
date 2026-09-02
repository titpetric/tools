package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/titpetric/tools/splint"
	"github.com/titpetric/tools/splint/analyzer"
	"github.com/titpetric/tools/splint/linters"
	"github.com/titpetric/tools/splint/simpleparser"
)

// saveFile is the document --save writes. It sits with the tree it describes,
// so a run reading another tree writes the file of that tree.
const saveFile = "splint.json"

// config is what one run was asked for.
type config struct {
	// options are what the parser is given.
	options splint.Options

	// parser names which parser reads the tree. The ast parser is the default
	// and stays the default: it is the exact reading, and the quick one is
	// something a caller asks for on purpose.
	parser string

	// input reads a document back from a file instead of parsing, and output
	// writes the parsed one to a file as well as linting it. Both are
	// resolved against the directory the command was run in.
	input  string
	output string

	// schema writes the document as a JSON Schema instead of linting it, and
	// stats writes what the linters measured instead of what they found.
	schema bool
	stats  bool

	// offline keeps the run off the network. What a module weighs is then
	// read from the size cache alone.
	offline bool

	// stripPrefix are package prefixes to take off a schema definition name.
	stripPrefix []string

	// linters selects the linters by name, and is every linter when empty.
	linters []string

	// json and yaml write the data the rendering would have drawn, and skip
	// the rendering. They are one question asked in two encodings.
	json bool
	yaml bool

	// save writes the parsed document to splint.json beside the tree, and
	// flags is the parser the help page is written from.
	save  bool
	flags *flag.FlagSet

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
	fs.StringVar(&cfg.options.SourcePath, "i", cfg.options.SourcePath, "read the tree at `PATH`")
	fs.StringVar(&cfg.parser, "parser", cfg.parser, "read the tree with `NAME`: "+analyzer.ParserName+" or "+simpleparser.ParserName)
	fs.StringVar(&cfg.input, "input", "", "read the document at `FILE` instead of parsing a tree")
	fs.StringVar(&cfg.output, "output", "", "write the parsed document to `FILE`")
	fs.BoolVar(&cfg.save, "save", false, "write the parsed document to "+saveFile+", beside the tree it describes")
	fs.BoolVar(&cfg.schema, "schema", false, "write the document as a JSON Schema instead of linting it")
	fs.BoolVar(&cfg.stats, "stats", false, "write what the linters measured instead of what they found")
	fs.BoolVar(&cfg.offline, "offline", false, "do not ask the module proxy, and read the sizes from the cache")
	fs.StringVar(&strip, "strip-prefix", "", "strip the package prefixes in `LIST` from schema names, comma separated")
	fs.StringVar(&selected, "linters", "", "run the linters in `LIST`, comma separated: "+strings.Join(linters.Names(), ", "))
	fs.BoolVar(&cfg.json, "json", false, "write the findings or the measurements as JSON")
	fs.BoolVar(&cfg.yaml, "yaml", false, "write the findings or the measurements as YAML")
	fs.BoolVar(&cfg.options.IncludeTests, "include-tests", false, "read the test files too")
	fs.BoolVar(&cfg.options.IncludeSources, "include-sources", false, "keep the source of every declaration")
	fs.BoolVar(&cfg.options.Verbose, "v", false, "say what is being read")
	fs.BoolVar(&cfg.help, "help", false, "print this help")

	if err := fs.Parse(reorder(fs, args)); err != nil {
		if err == flag.ErrHelp {
			cfg.help = true
			return cfg, nil
		}
		return nil, err
	}

	if cfg.json && cfg.yaml {
		return nil, fmt.Errorf("-json and -yaml are two encodings of one answer: ask for one")
	}

	if rest := fs.Args(); len(rest) > 0 {
		cfg.options.Pattern = rest[len(rest)-1]
	}
	cfg.linters = commaList(selected)
	cfg.stripPrefix = commaList(strip)
	cfg.flags = fs

	// --save writes the document beside the tree it describes. --output names
	// its own file, resolved against the directory the command was run in,
	// which is where the parse leaves the process.
	//
	// A document is read when --input names one. A splint.json found beside
	// the tree used to be read instead of parsing, and a run that had written
	// one earlier got that document back rather than the tree in front of it.
	if cfg.save && cfg.output == "" {
		cfg.output = filepath.Join(cfg.options.SourcePath, saveFile)
	}

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

// helpSpec is the page the command prints.
func helpSpec(cfg *config) spec {
	return spec{
		Name:    "splint",
		Tagline: "a linting framework over a data model of Go source",
		Usage: []string{
			"splint [flags] [pattern]",
		},
		Description: `The pattern is "." for the package in the source path and "./..." for
everything below it, which is how every other tool here spells it.

What is written depends on who is reading. A terminal gets a summary of what
each linter found and then the findings, in colour. Anything else gets one
GitHub Actions workflow command per finding, which is what puts one on the
file and the line of a pull request review.

The ast parser is the default. The simple parser reads the source without
building a syntax tree: it produces the same document, reads source that does
not compile, and is an order of magnitude quicker.`,
		Flags: cfg.flags,
		Examples: []example{
			{"splint ./...", "lint everything below here"},
			{"splint --save ./...", "lint, and write the parsed document to " + saveFile},
			{"splint --input " + saveFile, "lint a document read back, without parsing the tree"},
			{"splint -stats ./...", "what the linters measured, rather than what they found"},
			{"splint --json ./...", "the findings as data, for a program to read"},
			{"splint --linters godoc,imports ./...", "run two of the twelve"},
			{"splint --offline ./...", "read what a module weighs from the cache, and ask nobody"},
		},
		Notes: `Every run parses the tree. --input is the one way to read a document that was
already written, so a run is never answered by a file left behind by an
earlier one.

--save writes ` + saveFile + ` beside the tree and --output names another file.
Both are resolved against the directory the command was run in, whichever
tree -i pointed the parse at.

Exits 1 when a linter found something, 2 when the run itself failed.`,
	}
}

// reorder puts the flags of a command line in front of its operands.
//
// The flag package stops reading flags at the first argument that is not one,
// so "splint ./... --json" takes the pattern and then reads --json as a second
// operand: the run is silently not the one that was asked for, and the pattern
// ends up being the flag. Every other tool takes the two in either order, so
// this does too.
//
// A flag written as "-name value" carries the value with it, and a bool is
// written alone. The flag set knows which is which, which is what is asked
// here rather than guessed.
func reorder(fs *flag.FlagSet, args []string) []string {
	var flags, operands []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Everything after a bare -- is an operand, whatever it looks like.
		if arg == "--" {
			return append(flags, append(operands, args[i+1:]...)...)
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			operands = append(operands, arg)
			continue
		}

		flags = append(flags, arg)

		name := strings.TrimLeft(arg, "-")
		if strings.Contains(name, "=") || isBoolFlag(fs, name) {
			continue
		}
		if i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}

	return append(flags, operands...)
}

// isBoolFlag reports a flag that takes no value, which is what the flag
// package asks of a value to write it as "-name" alone.
func isBoolFlag(fs *flag.FlagSet, name string) bool {
	found := fs.Lookup(name)
	if found == nil {
		return false
	}

	boolean, ok := found.Value.(interface{ IsBoolFlag() bool })
	return ok && boolean.IsBoolFlag()
}
