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
splint -stats ./...                   # what the linters measured, not what they found
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
cli                   6      130ms        5ms    24.9x
oida                132      488ms       24ms    20.3x
atkins              276     1.897s       59ms    31.9x
phpscript           318     3.543s       81ms    43.5x
total                      14.459s      467ms    30.9x
```

Sixteen repositories, 1,778 Go files, and the whole sweep goes from fourteen
seconds to under half a second. The ratio widens with the tree, which is what
you would expect: the ast parser resolves a package against everything it
imports, and a line scan does not.

`task profile` takes ten seconds of parsing with a cpu and a memory profile
and prints the top of both. What it found, in order of what it was worth:

| Was | Cost |
|---|---|
| The branch keywords were counted with one pass over the line per keyword | 24% of cpu |
| The build constraint split the whole file to read a line from its header | 10% of allocations |
| `StringSet.Map` compiled its version regexp once per import | 7% of allocations |
| `strip` copied every line out and back, and most lines have nothing to blank | 12% of allocations |
| The line facts were four parallel slices | one allocation per file each |
| Every signature was joined into a new string, and most are one line already | |
| `declaredNames` parsed every signature to read the name off the front of it | |
| `selectors` returned a slice per line of every body, and the caller kept none | |

Together: **76ms to 45ms, 22.1MB to 15.3MB, and 225 thousand allocations down
to 140 thousand**, on the same tree with the same output. The parity harness
ran unchanged over the lot, which is what says the output is the same.

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
| `gomod/` | reads a go.mod into the model, and catalogues what it requires |
| `modproxy/` | asks the Go module proxy what a dependency weighs and how old it is |
| `loader/` | reads a document back from `.json` or `.yml` |
| `linters/` | the registry, one subpackage per linter |
| `schema/` | renders a document as a JSON Schema |
| `report/` | what was found: the issues, sorted and counted |
| `render/` | how it looks: drawn tables, markdown, GitHub lines |
| `cmd/splint` | the command |
| `tests/` | the parity harness and the benchmark |

The dependency runs one way. `model` imports nothing of its own; `splint`
imports `model`; the parsers import `splint` and `model`; the linters import
`model` alone; `cmd` imports all of it.

`report` holds what was found and `render` holds how it looks, so there is one
package to open when the output is wrong. The drawn tables are worktree's,
ported rather than imported: worktree draws them in `package main`, and splint
has no business depending on a CLI tool's module.

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

type Results struct {
	findings []Result
	packages map[string]*Metric   // what it counted, per package
}

func (r Results) Linter() string                 { return Name }
func (r Results) Len() int                       { return len(r.findings) }
func (r Results) All() iter.Seq[model.Issue]     { ... }
func (r Results) Metrics() model.LintMetrics     { ... }
func (r Results) Statistics() []model.Statistics { ... }

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

`Statistics` is one table: labels, rows, and two options for the line above and
the line below.

```go
model.NewStatistics(
	[]string{"Package", "Exported", "Documented"},
	rows,
	model.HeaderText("Documentation of every exported symbol, by package."),
	model.FooterText("14 of 18 exported symbols documented, 77.8%, 4 issues."),
)
```

A linter that measures nothing returns the zero `LintMetrics` and no tables,
and the renderer leaves it out rather than drawing an empty box under a
heading.

## The fixture

`testdata/` is a module of its own holding code that fails every check on
purpose: a file with no test beside it, an exported symbol with no doc, a
handler with no wrapper, symbols in files not named for them, two files
reaching different modules as `model`, a function taking `(time.Duration,
string)` and one returning `(error, *User)`.

Every parser skips `testdata`, `vendor`, `node_modules` and anything opening on
a dot or an underscore, so nothing reads it by accident. It is reached by being
pointed at:

```shell
splint -i testdata ./...
splint -stats -i testdata ./...
```

The root is read whatever it is called, which is what makes that work: a walk
that skipped the directory it was handed would read nothing at all.

## The linters

Ten of them. Four were written here; six came from gofsck, reimplemented
against the model rather than translated from its AST walk.

| Name | What it reports | What it measures |
|---|---|---|
| `godoc` | an exported symbol with no doc comment, one that does not open on the symbol it documents, one that does not end in punctuation, and one long enough to say the symbol does too much | documented against exported, per package |
| `imports` | two files of one package reaching different modules under the same short name, which compiles and reads as though they agree | import names and collisions, per package |
| `func-args` | a two argument function whose arguments read in an order a caller has to look up: a context that is not first, a duration that is not last, two parameters of the same type, an interface after a struct | funcs considered against passing |
| `func-returns` | a function returning an error or a bool before the value it qualifies | funcs considered against passing |
| `pairing` | a file with no test named after it | files, tests, paired and standalone, per package |
| `coverage` | an exported symbol no test is named for | covered against exported, and the constructors, per package |
| `grouping` | an exported symbol in a file not named for it | symbols passing, per package |
| `wraphandler` | an exported HTTP handler with no unexported function behind it, so only a server can call it | handlers wrapped, per package |
| `filecheck` | a file long enough to be doing more than one thing | line counts and their spread, **per file** |
| `visibility` | nothing | exported against internal, and the share of a package its private half occupies |
| `modcheck` | a replace directive, a requirement nothing imports, two majors of one module, a dependency reached from one file through one symbol | every dependency: size, files, packages, symbols used, direct or indirect, test only |

`func-args` considers a function taking exactly two arguments. The order of one
pair is unambiguous; for three or more the expected order is a heuristic sort,
and a heuristic reports too much to be worth reading.

`visibility` reports no issues at all, which the interface allows: an empty
report and a table. The counts are reported and not judged, because there is no
share of internal code a package ought to carry. A parser is mostly private and
a data model mostly not, and both are as they should be.

`modcheck` is the one linter that reaches outside the document. A size and a
published date are properties of the artifact rather than of the source, so
they come from the Go module proxy, and asking is the point of the check rather
than an option on it. Nothing is downloaded: the size is the `Content-Length`
of a `HEAD` on the module zip, and the version dates are one small JSON
document each. A machine with no network reports the coupling and leaves the
rest blank.

Size is the shallow half of what a dependency costs. The half that decides
whether it can be removed is how far it reaches: files importing it, packages
they belong to, and how many of its symbols are used.

```
| Import                   | Version   | Size     | Files | Pkgs | Symbols | Kind   | Behind |
|--------------------------|-----------|----------|-------|------|---------|--------|--------|
| github.com/a-h/templ     | v0.3.1020 | 1.9 MB   | 25    | 1    | 23      | direct |        |
| github.com/go-chi/chi/v5 | v5.3.2    | 130.0 KB | 4     | 3    | 5       | direct |        |
| golang.org/x/crypto      | v0.55.0   | 2.2 MB   | 5     | 5    | 4       | direct |        |
```

Those three weigh much the same and cost nothing like the same to remove. A
dependency with zero symbols and one file is imported for what it registers,
which is how a database driver is meant to be imported, so it is not reported
as thin.

This replaces the audit output of
[modcheck](https://github.com/titpetric/exp/tree/main/cmd/modcheck).

## Statistics

Every linter reports what it measured as well as what it found, because a check
that counts what it looked at has the count in hand by the time it knows what
to report. `-stats` prints those and nothing else, one table per linter, a
blank line apart, each with a line above saying what it is and a line below
summarising it.

```
Documentation of every exported symbol, by package.

| Package                            | Exported | Documented | Share  | Missing | Format | Verbose |
|------------------------------------|----------|------------|--------|---------|--------|---------|
| example.com/fixture                | 5        | 2          | 40.0%  | 1       | 2      | 0       |
| example.com/fixture/handler        | 2        | 2          | 100.0% | 0       | 0      | 0       |

14 of 18 exported symbols documented, 77.8%, 4 issues.
```

A linter picks its own columns, because what is worth a column is what it
measured. The numbers behind them are `model.LintMetrics`, keyed by package or
by file, holding the linter's own metric type: `filecheck` measures files and
everything else measures packages.

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
