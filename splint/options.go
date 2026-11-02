package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/pflag"

	"github.com/titpetric/tools/splint/commands/diff"
	"github.com/titpetric/tools/splint/commands/docs"
	"github.com/titpetric/tools/splint/linters"
	"github.com/titpetric/tools/splint/parsers/analyzer"
	"github.com/titpetric/tools/splint/parsers/simpleparser"
	"github.com/titpetric/tools/splint/pkg/splint"
)

// saveFile is the document --save writes. It sits with the tree it describes,
// so a run reading another tree writes the file of that tree.
const saveFile = "splint.json"

// The commands the tool takes. A command line naming none lints, which is
// what splint has always done and what every pipeline calling it expects.
const (
	commandLint     = "lint"
	commandFix      = "fix"
	commandDocs     = "docs"
	commandCoverage = "coverage"
	commandDiff     = "diff"
	commandSizes    = "sizes"
	commandTest     = "test"
)

// config holds the request for one run.
type config struct {
	// command is the verb the command line opened with: lint, which reports,
	// or fix, which rewrites.
	command string

	// fix rewrites the tree before linting it, so a report is of the tree as
	// the fixer left it and never of the one it found.
	fix bool

	// options are what the run passes to the parser.
	options splint.Options

	// parser names which parser reads the tree. The ast parser is the default
	// and stays the default: it reads the tree exactly, and a caller names the
	// quick one explicitly.
	//
	// parserNamed reports that the command line set the flag. The fixer picks
	// its own default when the flag is unset.
	parser      string
	parserNamed bool

	// input reads a document back from a file and skips the parse, and output
	// writes the parsed one to a file as well as linting it. The run resolves
	// both against the directory the command started in.
	input  string
	output string

	// coverageProfile names a Go coverage profile to fold into the parse. It
	// belongs to parsing a tree, so a run refuses it together with input.
	coverageProfile string

	// includeTests is the flag from the command line, and it decides whether the
	// written document keeps the test packages. The parse reads them either
	// way, because the linters have nothing to check without them.
	includeTests bool

	// schema writes the document as a JSON Schema and skips the lint, and
	// stats writes what the linters measured and omits what they found.
	schema bool
	stats  bool

	// render names the rendering splint docs writes, and split writes one
	// markdown file per package under out.
	render string
	split  bool
	out    string

	// hide are type names the puml diagram leaves out, and modelMode draws
	// the data model alone: no functions and no interfaces.
	hide      []string
	modelMode bool

	// docsRoot opens the documentation on one type and renders it as a
	// configuration reference, and docsTitle replaces that type's heading.
	docsRoot  string
	docsTitle string

	// docsSymbol names one symbol splint docs prints alone, godoc style: a
	// trailing "Open" or "Client.Close" argument.
	docsSymbol string

	// template names a text/template file the coverage report renders
	// through, with .Functions and .Packages as markdown tables.
	template string

	// oldFile and newFile are the two documents splint diff compares, and
	// the diffOptions are what the comparison includes.
	oldFile     string
	newFile     string
	diffOptions diff.Options

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
	// flags is the parser the help page reads.
	save  bool
	flags *pflag.FlagSet

	help bool
}

// parseOptions reads the command line.
//
// The trailing argument is the pattern, so "splint ./..." reads the whole tree
// and "splint ." reads one package, the same spelling every other tool here
// uses.
func parseOptions(args []string) (*config, error) {
	name, args := verb(args)
	cfg := &config{command: name, options: splint.NewOptions(), parser: analyzer.ParserName}

	var selected, strip, hide string
	fs := pflag.NewFlagSet("splint", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVarP(&cfg.options.SourcePath, "i", "i", cfg.options.SourcePath, "read the tree at `PATH`")
	fs.StringVar(&cfg.parser, "parser", cfg.parser, "read the tree with `NAME`: "+analyzer.ParserName+" or "+simpleparser.ParserName)
	fs.StringVar(&cfg.input, "input", "", "read the document at `FILE` and skip the parse")
	fs.StringVar(&cfg.output, "output", "", "write the parsed document to `FILE`")
	fs.BoolVar(&cfg.fix, "fix", false, "rewrite the import block of every file that needs one and format every file, then lint what is left")
	fs.BoolVar(&cfg.save, "save", false, "write the parsed document to "+saveFile+" beside the tree, and run no linter")
	fs.StringVar(&cfg.coverageProfile, "append-coverage", "", "fold the Go coverage profile at `FILE` into the parsed document")
	fs.BoolVar(&cfg.schema, "schema", false, "write the document as a JSON Schema and skip the linters")
	fs.BoolVar(&cfg.stats, "stats", false, "write what the linters measured")
	fs.BoolVar(&cfg.offline, "offline", false, "do not ask the module proxy, and read the sizes from the cache")
	fs.StringVar(&strip, "strip-prefix", "", "strip the package prefixes in `LIST` from schema and split file names, comma separated")
	fs.StringVar(&cfg.render, "render", docs.FormatMarkdown, "write the docs as `FORMAT`: markdown, spec, imports, puml or json; sizes takes json or d2")
	fs.BoolVar(&cfg.split, "split", false, "write the docs as one markdown file per package under --out")
	fs.StringVar(&cfg.out, "out", ".", "write the split docs under `DIR`")
	fs.StringVar(&hide, "hide", "", "leave the types in `LIST` out of the puml diagram, comma separated")
	fs.StringVar(&cfg.docsRoot, "root", "", "open the docs on the type `NAME` and render them as a config reference")
	fs.StringVar(&cfg.docsTitle, "title", "", "print `TEXT` as the heading of the root type")
	fs.BoolVar(&cfg.modelMode, "model", false, "draw the puml data model alone: no functions and no interfaces")
	fs.StringVar(&cfg.template, "template", "", "render the coverage report through the text/template at `FILE`")
	fs.StringVar(&cfg.oldFile, "old", "", "diff: the document of the older revision at `FILE`")
	fs.StringVar(&cfg.newFile, "new", "", "diff: the document of the newer revision at `FILE`")
	fs.BoolVar(&cfg.diffOptions.IncludeInternal, "include-internal", false, "diff: compare the internal packages as well")
	fs.BoolVar(&cfg.diffOptions.IncludeIndirect, "include-indirect", false, "diff: compare the indirect go.mod requirements as well")
	fs.BoolVar(&cfg.diffOptions.IncludeUnexported, "include-unexported", false, "diff: compare the unexported declarations as well, reported but never breaking")
	fs.StringVar(&selected, "linters", "", "run the linters in `LIST`, comma separated: "+strings.Join(linters.Names(), ", "))
	fs.BoolVar(&cfg.json, "json", false, "write the findings or the measurements as JSON")
	fs.BoolVar(&cfg.yaml, "yaml", false, "write the findings or the measurements as YAML")
	fs.BoolVar(&cfg.includeTests, "include-tests", false, "keep the test packages and the test files in the written document")
	fs.BoolVar(&cfg.options.IncludeSources, "include-sources", false, "keep the source of every declaration")
	fs.BoolVarP(&cfg.options.Verbose, "verbose", "v", false, "say what is being read")
	fs.BoolVarP(&cfg.help, "help", "h", false, "print this help")

	// pflag reads flags and operands in either order, so "splint ./... --json"
	// is the run "splint --json ./..." is.
	if err := fs.Parse(args); err != nil {
		if err == pflag.ErrHelp {
			cfg.help = true
			return cfg, nil
		}
		return nil, err
	}

	fs.Visit(func(given *pflag.Flag) {
		if given.Name == "parser" {
			cfg.parserNamed = true
		}
	})

	if cfg.json && cfg.yaml {
		return nil, fmt.Errorf("--json and --yaml are two encodings of one answer: ask for one")
	}

	// The overlay is part of extracting, as "append" says: it reads the line
	// ranges the parse recorded and writes coverage onto them. A document named
	// by -input comes from an earlier extract, and appending to it would mean
	// rewriting a file the run only reads.
	if cfg.coverageProfile != "" && cfg.input != "" {
		return nil, fmt.Errorf("--append-coverage folds a profile into a parse, and --input reads a saved document: ask for one")
	}

	// The fixer writes the files that make up a tree. A document named by
	// -input describes a tree the run never looked at, and rewriting from one
	// would edit the tree the process happens to be standing in.
	if (cfg.fix || cfg.command == commandFix) && cfg.input != "" {
		return nil, fmt.Errorf("a fix rewrites the tree and --input reads a saved document: ask for one")
	}

	// A diff is between two documents already written, so there is nothing
	// to compare until the command line names both.
	if cfg.command == commandDiff && (cfg.oldFile == "" || cfg.newFile == "") {
		return nil, fmt.Errorf("splint diff compares two documents: both --old and --new are required")
	}

	// The trailing argument is the pattern. splint docs also takes a symbol
	// after it, and tells a lone symbol from a lone pattern the way go doc
	// does: no separator and an uppercase first letter is a symbol.
	if rest := fs.Args(); len(rest) > 0 {
		last := rest[len(rest)-1]
		switch {
		case cfg.command == commandDocs && len(rest) > 1:
			cfg.docsSymbol = last
			cfg.options.Pattern = rest[len(rest)-2]
		case cfg.command == commandDocs && symbolQuery(last):
			cfg.docsSymbol = last
		default:
			cfg.options.Pattern = last
		}
	}
	cfg.linters = commaList(selected)
	cfg.stripPrefix = commaList(strip)
	cfg.hide = commaList(hide)
	cfg.flags = fs

	// --save writes the document beside the tree it describes. --output names
	// its own file, resolved against the directory the command started in;
	// the parse leaves the process there.
	//
	// The run reads a document only when --input names one. A splint.json beside
	// the tree does not replace the parse, so a run always reports the tree in
	// front of it.
	if cfg.save && cfg.output == "" {
		cfg.output = filepath.Join(cfg.options.SourcePath, saveFile)
	}

	// --save is the extract: the run is there for the document it writes, so
	// no linter runs and the exit code says nothing about the tree.
	if cfg.save {
		cfg.linters = []string{"none"}
	}

	// The schema comes from the types of a tree, and a type's declaration
	// describes it, so the sources come along whether the command line names
	// them or not. The docs print the declarations and the examples, which are
	// source too.
	if cfg.schema || cfg.command == commandDocs {
		cfg.options.IncludeSources = true
	}

	// A check that pairs a file with its test, or looks for the test of a
	// symbol, has nothing to read unless the parse reads the tests too, so every
	// parse reads them. Whether they survive into the document a run writes is
	// what --include-tests decides: a consumer rendering a report off one
	// listed a _test package as a package, which is not a package anyone
	// wrote.
	cfg.options.IncludeTests = true

	// The import rules read the declarations as a file writes them, and the
	// fixer writes them back. Every run of the command requests them: they are
	// a walk of lines the parse already makes, and a run without them would
	// report every file as holding no imports at all.
	cfg.options.IncludeImports = true

	// The integration rule reads what a test needs to run: the literals it
	// passes its code and the files kept behind a build tag. The walk is over
	// the test files alone, which a parse reads anyway.
	cfg.options.IncludeTestFacts = true

	return cfg, nil
}

// symbolQuery reports an argument naming a symbol and not a tree: no path
// separator and an uppercase first letter, the same test go doc uses to tell
// the two apart. "Config" and "Config.IfDirective" are symbols; ".", "./..."
// and a directory are patterns.
func symbolQuery(arg string) bool {
	if strings.Contains(arg, "/") {
		return false
	}
	r, _ := utf8.DecodeRuneInString(arg)
	return unicode.IsUpper(r)
}

// verb peels a leading command off the command line.
//
// A command line that opens with anything else is a lint, so "splint ./..."
// and every pipeline written before splint had commands keep their meaning.
func verb(args []string) (string, []string) {
	if len(args) == 0 {
		return commandLint, args
	}

	switch args[0] {
	case commandFix, commandLint, commandDocs, commandCoverage, commandDiff, commandSizes:
		return args[0], args[1:]
	}

	return commandLint, args
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
			"splint fix [flags] [pattern]",
			"splint docs [flags] [pattern] [Symbol[.Method]]",
			"splint coverage [flags] [pattern]",
			"splint diff --old FILE --new FILE [flags]",
			"splint test [flags] [pattern] [-- go test flags]",
		},
		Commands: []command{
			{commandLint, "report what the linters found. This is what a command line naming no verb does"},
			{commandFix, "rewrite the import block of every file that does not hold the one the rules describe, and format every file"},
			{commandDocs, "render the tree as an API reference: markdown, a spec, an import list or plantuml"},
			{commandCoverage, "report the coverage the document carries, per function and per package"},
			{commandDiff, "compare the exported API and the go.mod of two documents"},
			{commandSizes, "report every file with its byte size, the per directory totals and a histogram, as JSON"},
			{commandTest, "run the Go tests a change reaches, comparing the working tree with HEAD"},
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
			{"splint fix ./...", "rewrite every import block to the house rules, and report nothing"},
			{"splint --fix ./...", "rewrite them, then report what is left"},
			{"splint --save ./...", "write the parsed document to " + saveFile + ", and lint nothing"},
			{"splint --input " + saveFile, "lint a document read back, without parsing the tree"},
			{"splint --save --append-coverage=pkg.cov ./...", "write the document with the coverage of every function in it"},
			{"splint docs ./... > docs/api.md", "the API reference of the tree"},
			{"splint docs --render puml ./...", "a plantuml diagram of the types"},
			{"splint docs --split --out docs/api --strip-prefix github.com/titpetric ./...", "one markdown file per package"},
			{"splint docs --root Config --title \"# Configuration\" .", "a config reference from one type down"},
			{"splint docs . Client.Close", "one symbol, godoc style, and how many tests reach it"},
			{"splint docs -v ./... Open", "the same, listing every reference to it, tests and product code"},
			{"splint coverage --append-coverage=pkg.cov ./...", "fold a profile into a parse and report it"},
			{"splint coverage --input " + saveFile + " --template docs/testing-coverage.md.tpl", "the report of a document already written"},
			{"splint diff --old old.json --new new.json", "what a release takes away"},
			{"splint --linters none --output model.json ./...", "extract a document and judge nothing"},
			{"splint --stats ./...", "what the linters measured"},
			{"splint --json ./...", "the findings as data, for a program to read"},
			{"splint --linters godoc,imports ./...", "run two of the thirteen"},
			{"splint --offline ./...", "read what a module weighs from the cache, and ask nobody"},
			{"splint test ./...", "run the tests the working tree changed"},
			{"splint test --split ./pkg", "one go test invocation per changed test in a package"},
			{"splint test --base origin/main ./...", "run the tests a branch changed, against main"},
			{"splint test ./... -count=1 -timeout 2m", "forward flags to every go test invocation"},
		},
		Notes: `splint fix rewrites the import block of every file that does not hold the
one the rules describe: the groups in order, one declaration per file, an
import nothing reaches taken out, and an import a name needs put in. It reads
.splint.yml beside the tree for the group order, the company prefixes and the
names the tree cannot place on its own, and it counts what it rewrote under
imports.fixed in splint.yml under the user configuration directory, which is
where a count of what this machine has done belongs rather than beside the
tree. It never writes go.mod and never touches a generated file. A file holding a name nothing can place is left alone, and
splint ./... says which name.

Every file it reads is formatted, which is gofmt through go/format: the block
is this command's rule and the rest of the file is gofmt's, and nothing else
here runs it. A run says which files it rewrote and which it only formatted. A
file that does not parse keeps its block and is not formatted.

The fixer reads the tree with the simple parser unless --parser names one: a
file that is missing an import it needs does not compile, and that is the file
the fixer is there to repair. A file that imports nothing is valid Go and is
left alone.

splint docs renders the tree as an API reference instead of linting it: the
package godoc, every exported declaration behind a details fold, and the
godoc examples printed whole. splint coverage reports the coverage a document
carries, which --append-coverage folds in; --template renders a page from the
.Functions and .Packages tables instead of printing the function table.

Every run parses the tree. --input is the one way to read a document that was
already written, so a run is never answered by a file left behind by an
earlier one.

--save writes ` + saveFile + ` beside the tree and --output names another file.
Both are resolved against the directory the command was run in, whichever
tree -i pointed the parse at.

splint test compares the working tree with HEAD, or the revision --base names,
and keeps the functions whose signature or body moved. A changed test runs
itself, a changed benchmark runs under -benchtime 10s, and a changed function
runs the tests that name it: Test<Func>, and for a method on type T every
Test<T>*. A test that reaches the function through a helper of its own package
runs too. Doc comments and type declarations are not changes: go build judges
a type, and a comment changes no behaviour. A flag splint does not know is
forwarded to every go test invocation, and --split runs every test in an
invocation of its own. The run writes one PASS or FAIL line per test.

Exits 1 when a linter found something, 2 when the run itself failed.`,
	}
}
