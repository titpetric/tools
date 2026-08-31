# splint

A data model of Go source, two parsers that fill it, and a linting framework
over the top.

The model is schema and nothing else: it imports no third party package, so a
linter written against it drags in neither `go/ast` nor `x/tools`. A parser
fills it and a linter reads it, and neither knows the other exists.

## Install

```bash
go install github.com/titpetric/tools/splint/cmd/splint@latest
```

## Use

```bash
splint ./...                          # lint everything below here
splint -i ../oida ./...               # lint another tree
splint --parser=simpleparser ./...    # read it without building a syntax tree
splint --linters godoc,imports ./...  # run two of the four
splint --output model.json ./...      # keep the document the linters read
splint --input model.json             # lint a document read back from a file
splint --format github ./...          # one line per issue, for a CI log
splint --schema ./...                 # write the tree as a JSON Schema
```

It exits 1 when a linter found something and 2 when the run itself failed, so a
pipeline can tell a finding from a failure.

Output is drawn for a terminal and written as a markdown table for anything
else, so `splint ./... > REPORT.md` produces something to paste into a
document. The table is padded the way `mdox fmt` pads one, which means a
document holding it is not reformatted the next time the docs are built.

```
3 issues from 1 linter: godoc 3.

| Position                        | Severity | Rule          | Symbol           | Message                                                             |
|---------------------------------|----------|---------------|------------------|---------------------------------------------------------------------|
| internal/options_from_env.go:29 | WARN     | godoc/verbose | OptionsFromEnv   | godoc runs to 11 lines, which usually says the symbol does too much |
| serve_http.go:22                | WARN     | godoc/verbose | Tracer.ServeHTTP | godoc runs to 11 lines, which usually says the symbol does too much |
| start_auto.go:20                | WARN     | godoc/verbose | StartAuto        | godoc runs to 11 lines, which usually says the symbol does too much |
```

Under `--format github` the same issues read as one line each, in the shape a
compiler writes and GitHub Actions resolves against a checkout:

```
internal/options_from_env.go:29: godoc/verbose: godoc runs to 11 lines, which usually says the symbol does too much
```

## The two parsers

Both are constructed as `New(splint.Options)` and both return a
`*model.DocumentRoot`, so which one a program uses is an import and nothing
more.

| | `astparser` | `simpleparser` |
|---|---|---|
| Reads through | `go/ast` and `x/tools` | bytes |
| Exact | yes, the toolchain resolves it | to within a rounding error, see below |
| Source that does not compile | depends on the toolchain | reads it regardless |
| Default | yes | on request |

`astparser` is the default and stays it: it is the exact reading, and the quick
one is something a caller asks for on purpose.

`simpleparser` finds a declaration by where it starts and where it ends. gofmt
puts every top level declaration at column zero and the brace or paren that
closes one at column zero as well, so a function's extent is found without
balancing a single brace inside it. That extent, taken verbatim, is the source
the model records.

It is the reason the model has to be free of `go/ast`: a parser that never
builds a tree cannot fill a schema that names one. It is also what makes
another language possible later, since nothing in the model is Go specific.

### How fast

`task bench` times both parsers by running the command over every project,
which is what a caller of it experiences rather than what a benchmark inside
the process measures.

```
project           files  astparser     simple    ratio
cli                   6      137ms        6ms    23.9x
oida                132      508ms       39ms    13.0x
platform-app        186     2.057s       97ms    21.1x
phpscript           318     3.673s      153ms    24.0x
total                      16.685s      788ms    21.2x
```

Sixteen repositories, 1,778 Go files, and the whole sweep goes from seventeen
seconds to eight hundred milliseconds.

### How close the two are

`task parity` runs the simple parser over sixteen repositories and compares
what it produced against what `go-fsck extract` produced for the same tree.

The comparison is exhaustive by construction. It walks the encoded documents
value by value rather than checking a list of fields somebody remembered: every
key either side carries is visited, a key only one of them has is a difference,
and a key added to the model tomorrow is covered without anyone adding it here.
The declaration lists are keyed on the symbol before the walk, so one extra
declaration is one difference rather than a shift that misreports every
declaration after it.

```
total: 192091 values compared, 1533 differ (0.7981%)
  Funcs[].Complexity.Cognitive   1400  gocognit weights a branch by how deeply it nests
  Funcs[].Globals                   2  unexplained
      ./renderer Funcs[renderer.go.Renderer.renderBlock].Globals: <absent> != {"decl":["Key"]}
  ...
unexplained: 133 of 192091 values, 0.0692%, budget 0.1000%
```

Cognitive complexity is the one field the two are not expected to agree on, and
it is nearly all of the gap: `gocognit` weights a branch by how deeply it nests
in the syntax tree, and a line scan has no tree to read the nesting from.
Everything else comes to **133 values in 192,091**, and the harness asserts that
as a budget: a change that pushes it higher fails the test rather than quietly
widening the gap. Each difference prints an example, so a number in the summary
is something a reader can go and look at.

Two differences are structural and are normalised rather than chased:

- **Column alignment.** go-fsck renders a declaration through `go/printer`,
  which writes the padding that lines up a run of struct fields as tabs. The
  file on disk carries what gofmt wrote, which is spaces. The two say the same
  thing, so the padding inside a line is collapsed before they are compared.
- **Package paths under a nested module.** `go-fsck extract` writes `.generic`
  for the root package of a module nested below the parse root, and
  `./frontend` for a package of the root module. The simple parser reproduces
  both, because a document that spelled them differently would not compare
  against one already on disk.

## Packages

| Package | What |
|---|---|
| `model/` | the schema, and the linter interfaces over it. No third party imports |
| `analyzer/` | the `go/ast` parser, moved from go-fsck |
| `simpleparser/` | the parser that reads bytes |
| `gomod/` | reads a go.mod into the model, which both parsers need |
| `loader/` | reads a document back from `.json` or `.yml` |
| `linters/` | the registry, one subpackage per linter |
| `schema/` | renders a document as a JSON Schema |
| `report/` | rendering: terminal, markdown, GitHub |
| `cmd/splint` | the command |
| `tests/` | the parity harness and the benchmark |

The dependency runs one way. `model` imports nothing of its own; `splint`
imports `model`; the parsers import `splint` and `model`; the linters import
`model` alone; `cmd` imports all of it.

## The model

`model.DocumentRoot` is what one parse produced: the packages it found and the
modules they belong to. Every field carries a `json` and a `yaml` tag naming the
same key, so the two encodings describe the same document and `loader` reads
either.

The JSON keys are the Go field names, which is what `go-fsck extract` writes.
That is deliberate: a document splint writes is one go-fsck's own subcommands
can read, and the other way round.

Nothing in the package marshals anything itself. What it does carry is
utilities over the data: `DeclarationList.Exported`, `Definition.Merge`,
`StringSet.Add`, `Declaration.Position`. A type with helpers hanging off it is
the shape to reach for; a type with a `MarshalJSON` is not.

## Writing a linter

A linter keeps its own result type and implements two methods on the slice of
them. The framework ranges over that slice in place and only ever materialises
the `model.Issue` view of each, so a rule carrying twenty fields per finding is
walked without copying any of them.

```go
package mine

const Name = "mine"

type Result struct {
	Rule     string
	Symbol   string
	Position model.Position
	Message  string
	// whatever else this rule has to say
}

func (r Result) Issue() model.Issue { ... }

type Results []Result

func (r Results) Linter() string           { return Name }
func (r Results) Len() int                 { return len(r) }
func (r Results) All() iter.Seq[model.Issue] { ... }

type Linter struct{}

func New() *Linter        { return &Linter{} }
func (l *Linter) Name() string { return Name }

func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	var results Results
	for _, def := range root.Packages {
		// ...
	}
	return results, nil
}
```

Add it to `linters.All()` and it runs. A linter never touches disk and never
parses anything: everything it needs is in the document, which is what lets the
same rule run over a tree either parser read, and over one loaded from a file.

An issue carries an `slog.Level` for its severity, a position that reads
`package: file.go:line`, the linter and rule it came from, and an `Attrs` map
for whatever else the rule has to say. The map is what keeps the schema from
growing a field per rule.

## The linters

| Name | What it reports |
|---|---|
| `godoc` | an exported symbol with no doc comment, one that does not open on the symbol it documents, one that does not end in punctuation, and one long enough to say the symbol does too much |
| `imports` | two files of one package reaching different modules under the same short name, which compiles and reads as though they agree |
| `func-args` | a two argument function whose arguments read in an order a caller has to look up: a context that is not first, a duration that is not last, two parameters of the same type, an interface after a struct |
| `func-returns` | a function returning an error or a bool before the value it qualifies |

`func-args` considers a function taking exactly two arguments. The order of one
pair is unambiguous; for three or more the expected order is a heuristic sort,
and a heuristic reports too much to be worth reading.

## Schemas

`--schema` writes the types of a tree as a JSON Schema draft-07 document rather
than linting it. It reads a source tree or a document already written for one,
which is the same thing said twice:

```shell
splint --schema ./... > schema.json
splint --schema --input go-fsck.json > schema.json
splint --schema --strip-prefix example.com/ ./...
```

This was `go-fsck jsonschema` and reads better for the move: it renders every
package of a tree rather than the first one it was handed, and a type name two
packages both declare is qualified with the package, since a schema has one
namespace and two `Config` types are not the same thing. An interface and a
test package describe no shape and are left out.

## Development

`task` builds and tests everything. `task parity` prints how far the two
parsers are apart, value by value, and `task bench` how far apart they are in
time.

`SPLINT_WORKSPACE` points the harness at where the projects it reads are
checked out. A project that is not there is skipped, so it reads whatever the
machine has.
