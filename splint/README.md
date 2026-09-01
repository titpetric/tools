# splint

A data model of Go source, two parsers that fill it, and a linting framework
over the top.

The model is schema and nothing else. It imports no third party package, so a
linter written against it links neither `go/ast` nor `x/tools`. A parser fills
the model and a linter reads it; neither refers to the other.

The command is built to run in a pipeline. It exits on a code a job branches
on, and it renders the one report three ways for the three readers a check has:
the pipeline that gates on it, the operator watching a terminal, and whoever
reads the artifact afterwards. Which rendering a run produces is decided by
where the output goes, so the same command line serves all three.

## Install

```bash
go install github.com/titpetric/tools/splint/cmd/splint@latest
```

## Use

```bash
splint ./...                          # lint everything below here
splint -i ../oida ./...               # lint another tree
splint --parser=simpleparser ./...    # read it without building a syntax tree
splint --linters godoc,imports ./...  # run two of the twelve
splint --output model.json ./...      # keep the document the linters read
splint --input model.json             # lint a document read back from a file
splint --format github ./...          # one line per issue, for a CI log
splint --schema ./...                 # write the tree as a JSON Schema
splint -stats ./...                   # what the linters measured, not what they found
splint --offline ./...                # never ask the module proxy
```

The exit code is 0 for a clean run, 1 when a linter reported something, and 2
when the run itself failed, so a pipeline can tell a finding from a failure.

## Output

The report is one set of issues in three renderings, one per reader:

| Rendering  | Reader                      | What it is                                             |
|------------|-----------------------------|--------------------------------------------------------|
| `terminal` | the operator watching a run | one drawn box per finding, two lines in it             |
| `markdown` | a PR comment, an artifact   | a padded markdown table, and the summary line above it |
| `github`   | the CI log                  | one line per issue, in the shape a compiler writes     |

`--format` names one and defaults to `auto`, under which the destination
decides. `render.IsTerminal` asks whether the writer is an `*os.File` on a
character device: stdout on a terminal gets the boxes, and a pipe, a file or a
CI log gets the markdown table. `TERM=dumb` counts as not a terminal, which
is what a pager or an editor sets when it wants plain text.

So `splint ./...` draws for whoever is watching and `splint ./... > REPORT.md`
writes a document to paste from, with no flag in either. The markdown table is
padded the way `mdox fmt` pads one, so a document holding it is not reformatted
the next time the docs are built.

The markdown rendering, which is what a redirect or a pipe produces:

3 issues from 1 linter: godoc 3.

| Position                        | Severity | Rule          | Symbol           | Message                                                             |
|---------------------------------|----------|---------------|------------------|---------------------------------------------------------------------|
| internal/options_from_env.go:29 | WARN     | godoc/verbose | OptionsFromEnv   | godoc runs to 11 lines, which usually says the symbol does too much |
| serve_http.go:22                | WARN     | godoc/verbose | Tracer.ServeHTTP | godoc runs to 11 lines, which usually says the symbol does too much |
| start_auto.go:20                | WARN     | godoc/verbose | StartAuto        | godoc runs to 11 lines, which usually says the symbol does too much |

The terminal rendering is one box per finding, a blank line apart, and two
lines in each: where it is, and what is wrong. A message is a sentence and a
position is a path, and a table of both is a table as wide as the terminal with
one column of it worth reading.

```
4 issues from 1 linter: godoc 4.

╭──────────────────────────────────────────────────────╮
│ WARN undocumented.go:9 (godoc/missing)               │
│ Undocumented - exported symbol lacks a godoc comment │
╰──────────────────────────────────────────────────────╯

╭──────────────────────────────────────────────────────────╮
│ WARN undocumented.go:5 (godoc/format)                    │
│ Thing - godoc should open on "Thing" and opens on "This" │
╰──────────────────────────────────────────────────────────╯
```

`ERROR` is red, `WARN` amber and `INFO` teal, the position is teal, the rule is
grey and the symbol is violet. The message carries no colour: it is the line a
reader stops on, and everything around it is what they scanned to get there. A
finding about a file rather than a symbol is the message on its own.

Under `--format github` the same issues are one line each, in the form a
compiler writes and GitHub Actions resolves against a checkout into an
annotation:

```
internal/options_from_env.go:29: godoc/verbose: godoc runs to 11 lines, which usually says the symbol does too much
```

`-stats` under `--format github` writes the markdown tables rather than lines:
a log reads lines, a table is not one, and the table is what a reader can still
take something from.

## The two parsers

Both are constructed as `New(splint.Options)` and both return a
`*model.DocumentRoot`, so which one a program uses is an import.

|                              | `astparser`                    | `simpleparser`               |
|------------------------------|--------------------------------|------------------------------|
| Reads through                | `go/ast` and `x/tools`         | bytes                        |
| Exact                        | yes, the toolchain resolves it | to the margin measured below |
| Source that does not compile | depends on the toolchain       | reads it regardless          |
| Default                      | yes                            | on request                   |

`simpleparser` finds a declaration by where it starts and where it ends. gofmt
puts every top level declaration at column zero, and the brace or paren that
closes one at column zero as well, so a function's extent is found without
balancing a brace inside it. That extent, taken verbatim, is the source the
model records.

A parser that builds no tree cannot fill a schema that names one, which is why
the model carries no `go/ast` types. Nothing in the model is Go specific.

### Speed

`task bench` times both parsers by running the command over every project,
which measures what a caller of the command experiences rather than what a
benchmark inside the process does.

```
project           files  astparser     simple    ratio
cli                   6      131ms        5ms    25.5x
oida                132      671ms       38ms    17.7x
atkins              276     2.523s      112ms    22.5x
phpscript           319     5.573s      138ms    40.5x
total                      18.999s      697ms    27.3x
```

Sixteen repositories, 1,843 Go files: 18.999s for the ast parser and 697ms for
the line scan. The ratio rises with the size of the tree. The ast parser
resolves a package against everything it imports; a line scan does not.

One linter runs in the timing, not all of them. `modcheck` asks the module
proxy about every dependency, which is a round trip per module and longer than
either parser takes.

### Profile

`task profile` takes ten seconds of parsing with a cpu and a memory profile and
prints the top of both. What it reported, in the order of what each was worth:

| Was                                                                           | Cost                         |
|-------------------------------------------------------------------------------|------------------------------|
| The branch keywords were counted with one pass over the line per keyword      | 24% of cpu                   |
| The build constraint split the whole file to read a line from its header      | 10% of allocations           |
| `StringSet.Map` compiled its version regexp once per import                   | 7% of allocations            |
| `strip` copied every line out and back, and most lines have nothing to blank  | 12% of allocations           |
| The line facts were four parallel slices                                      | one allocation per file each |
| Every signature was joined into a new string, and most are one line already   | not measured separately      |
| `declaredNames` parsed every signature to read the name off the front of it   | not measured separately      |
| `selectors` returned a slice per line of every body, and the caller kept none | not measured separately      |

Together: 76ms to 45ms, 22.1MB to 15.3MB, and 225 thousand allocations to 140
thousand, on the same tree with the same output. The parity harness ran
unchanged over the same repositories.

### Parity

`task parity` runs the simple parser over sixteen repositories and compares
what it produced against what `go-fsck extract` produced for the same tree.

The comparison walks the encoded documents value by value rather than checking
a list of fields: every key either side carries is visited, a key only one of
them has is a difference, and a key added to the model later is covered without
an edit here. The declaration lists are keyed on the symbol before the walk, so
one extra declaration is one difference rather than a shift that misreports
every declaration after it.

```
total: 200932 values compared, 1904 differ (0.9476%)
  Funcs[].Complexity.Cognitive   1426  gocognit weights a branch by how deeply it nests
  Funcs[].Globals                   2  unexplained
      ./renderer Funcs[renderer.go.Renderer.renderBlock].Globals: <absent> != {"decl":["Key"]}
  ...
unexplained: 133 of 200932 values, 0.0662%, budget 0.1000%
```

Cognitive complexity accounts for 1426 of the 1904: `gocognit` weights a branch
by how deeply it nests in the syntax tree, and a line scan has no tree to read
the nesting from. `Module.Sums` accounts for 332: go-fsck reads the go.mod and
not the go.sum beside it. The remainder is 133 values in 200,932, and the
harness asserts that as a budget: a change that pushes it higher fails the
test. Each difference prints an example.

Three differences are representational, and are handled rather than counted:

- **Column alignment.** go-fsck renders a declaration through `go/printer`,
  which writes the padding that lines up a run of struct fields as tabs. The
  file on disk carries what gofmt wrote, which is spaces. The padding inside a
  line is collapsed before the two are compared.
- **Package paths under a nested module.** `go-fsck extract` writes `.generic`
  for the root package of a module nested below the parse root, and
  `./frontend` for a package of the root module. The simple parser reproduces
  both, so a document it writes compares against one already on disk.
- **The blank import alias.** go-fsck records `_ "embed"` as `"embed"` and
  splint records the underscore, which `modcheck` reads. The alias is dropped
  from both sides before they are compared.

## Packages

| Package         | What                                                                                  |
|-----------------|---------------------------------------------------------------------------------------|
| `model/`        | the schema, and the linter interfaces over it. No third party imports                 |
| `analyzer/`     | the `go/ast` parser, moved from go-fsck                                               |
| `simpleparser/` | the parser that reads bytes                                                           |
| `gomod/`        | reads a go.mod and a go.sum into the model, and catalogues what they require          |
| `modproxy/`     | asks the Go module proxy what a dependency weighs, how old it is and what it requires |
| `loader/`       | reads a document back from `.json` or `.yml`                                          |
| `linters/`      | the registry, one subpackage per linter                                               |
| `schema/`       | renders a document as a JSON Schema                                                   |
| `report/`       | what was found: the issues, sorted and counted                                        |
| `render/`       | how it looks: drawn tables, markdown, GitHub lines                                    |
| `cmd/splint`    | the command                                                                           |
| `tests/`        | the parity harness and the benchmark                                                  |

The dependency runs one way. `model` imports nothing of its own; `splint`
imports `model`; the parsers import `splint` and `model`; the linters import
`model` alone; `cmd` imports all of it.

`report` holds what was found and `render` holds how it looks, so there is one
package to open when the output is wrong. The drawn tables are worktree's,
ported rather than imported: worktree draws them in `package main`, and splint
does not depend on a CLI tool's module.

## The model

`model.DocumentRoot` is what one parse produced: the packages it found and the
modules they belong to. Every field carries a `json` and a `yaml` tag naming the
same key, so the two encodings describe the same document and `loader` reads
either.

The JSON keys are the Go field names, which is what `go-fsck extract` writes. A
document splint writes is one go-fsck's own subcommands can read, and the other
way round.

Nothing in the package marshals anything itself. What it carries is utilities
over the data: `DeclarationList.Exported`, `Definition.Merge`, `StringSet.Add`,
`Declaration.Position`. A type with helpers hanging off it is the shape to
reach for; a type with a `MarshalJSON` is not.

## Writing a linter

A linter keeps its own result type and implements two methods on the slice of
them. The framework ranges over that slice in place and materialises only the
`model.Issue` view of each, so a rule carrying twenty fields per finding is
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
parses anything: everything it needs is in the document, so the same rule runs
over a tree either parser read and over one loaded from a file.

A linter does not read `Declaration.Source` either. A rule is a question about
the model, and a rule that has to read the text is a field the model is
missing: add the field to the parsers, both of them, and read that. The source
is in the document for consumers that are not linters, a browser or an analysis
that walks branches, and `splint ./...` does not keep it.

An issue carries an `slog.Level` for its severity, a position that reads
`package: file.go:line`, the linter and rule it came from, and an `Attrs` map
for anything else the rule has to say. The map is what keeps the schema from
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
reaching different modules as `model`, a blank import outside main.go, a file
whose every symbol reaches the file beside it, a function taking
`(time.Duration, string)` and one returning `(error, *User)`.

Every parser skips `testdata`, `vendor`, `node_modules` and anything opening on
a dot or an underscore, so nothing reads it by accident. It is reached by being
pointed at:

```shell
splint -i testdata ./...
splint -stats -i testdata ./...
```

The root is read whatever it is called. A walk that skipped the directory it
was handed would read nothing at all.

## The linters

Twelve of them. Six were written here; six came from gofsck, reimplemented
against the model rather than translated from its AST walk.

| Name            | What it reports                                                                                                                                                                                          | What it measures                                                                                                                                            |
|-----------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `godoc`         | an exported symbol with no doc comment, one that does not open on the symbol it documents, one that does not end in punctuation, and one long enough to say the symbol does too much                     | documented against exported, per package                                                                                                                    |
| `imports`       | two files of one package reaching different modules under the same short name, which compiles and reads as though they agree                                                                             | import names and collisions, per package                                                                                                                    |
| `func-args`     | a two argument function whose arguments read in an order a caller has to look up: a context that is not first, a duration that is not last, two parameters of the same type, an interface after a struct | funcs considered against passing                                                                                                                            |
| `func-returns`  | a function returning an error or a bool before the value it qualifies                                                                                                                                    | funcs considered against passing                                                                                                                            |
| `pairing`       | a file with no test named after it                                                                                                                                                                       | files, tests, paired and standalone, per package                                                                                                            |
| `coverage`      | an exported symbol no test is named for                                                                                                                                                                  | covered against exported, and the constructors, per package                                                                                                 |
| `grouping`      | an exported symbol in a file not named for it                                                                                                                                                            | symbols passing, per package                                                                                                                                |
| `wraphandler`   | an exported HTTP handler with no unexported function behind it, so only a server can call it                                                                                                             | handlers wrapped, per package                                                                                                                               |
| `filecheck`     | a file long enough to be doing more than one thing                                                                                                                                                       | line counts and their spread, **per file**                                                                                                                  |
| `visibility`    | nothing                                                                                                                                                                                                  | exported against internal, and the share of a package its private half occupies                                                                             |
| `selfcontained` | nothing                                                                                                                                                                                                  | what a file needs from the rest of its package: symbols, types and funcs reaching nothing outside the file they are in, and the share that do, **per file** |
| `modcheck`      | five module rules, listed below                                                                                                                                                                          | every dependency: size, reach, files, packages, symbols used, kind; and every version go.sum records                                                        |

`func-args` considers a function taking exactly two arguments. The order of one
pair is unambiguous; for three or more the expected order is a heuristic sort.

`visibility` and `selfcontained` report no issues at all, which the interface
allows: an empty report and a table. The counts are reported and not judged. A
parser is mostly private and a data model mostly not, and a package written as
one unit across several files is as legitimate as one written as several.

## modcheck

`modcheck` is the linter that reaches outside the document. A size, a
publication date and the requirements of a released version are properties of
the artifact rather than of the source, so they come from the Go module proxy.
Nothing is downloaded: the size is the `Content-Length` of a `HEAD` on the
module zip, the version dates are one small JSON document each, and the
requirements are the `.mod` file of that version. A machine with no network
reports the columns that come out of the document and leaves the rest blank.

It replaces the audit output of
[modcheck](https://github.com/titpetric/exp/tree/main/cmd/modcheck).

### The size cache

A module version is immutable, so what it weighs is a fact that does not
expire. The sizes are kept in `~/.cache/splint/sizes.yml`, under the cache
directory the machine names, and written once when the linter has finished
asking rather than per module:

```yaml
github.com/a-h/templ:
    size: 1961490
    versions:
        v0.3.1020: 1961490
```

`size` is the mean of the versions below it, and is what a version the file
does not hold is answered with. A module is much the same size from one release
to the next, so an answer within a few percent of the truth costs a round trip
less than the truth does.

The file is written beside itself and renamed over the old one, and what was
written is read back before the rename: a cache that does not parse is one the
next run would throw away, and throwing it away here costs nobody anything.

`--offline` asks nobody. The sizes come from the cache alone, and the columns
only the proxy can answer, the reach of a dependency and how far behind it is,
are left blank rather than reported as nothing. One run fills the cache and
every run after it can be offline.

```
splint --linters modcheck -stats ./...             # 3.1s cold, 1.8s with the sizes cached
splint --linters modcheck -stats --offline ./...   # 0.6s, asks nobody
```

### The rules

| Rule      | Severity | Reports                                                                                                            |
|-----------|----------|--------------------------------------------------------------------------------------------------------------------|
| `replace` | error    | a replace directive, so what the build resolves to is not what the go.mod requires                                 |
| `unused`  | warn     | a requirement no file imports                                                                                      |
| `majors`  | warn     | two majors of one module required together, which link both and whose types do not satisfy each other's interfaces |
| `thin`    | info     | a dependency reached from one file through one symbol                                                              |
| `blank`   | warn     | a blank import in a file other than main.go or main_test.go                                                        |

A blank import runs the package's init and reaches no symbol. What it does is
decided by which packages the binary links: a driver registers itself with
`database/sql`, `net/http/pprof` mounts handlers on the default mux. main.go
and main_test.go are where a program and a test binary are wired, and `TestMain`
is the entry point of the second. A blank import in any other file is reported,
and the finding names that file rather than the go.mod the other four rules
name. One row of it:

| Position              | Severity | Rule           | Symbol              | Message                                                                      |
|-----------------------|----------|----------------|---------------------|------------------------------------------------------------------------------|
| server/server_test.go | WARN     | modcheck/blank | example.com/drivers | example.com/drivers is imported for its side effect from server_test.go, ... |

Two are left alone: a package belonging to the tree being read, and `embed`.
Where a project puts its own registrations is its own arrangement, and a blank
`embed` is what the compiler asks for to embed into a string or a byte slice.

### What a dependency costs

Size is one column. The others are how far the dependency reaches into the tree
and how much of the build list it brings with it. Four rows of one run over
etl:

| Import                    | Version | Size     | Deps | Total    | Files | Pkgs | Symbols | Kind   | Behind |
|---------------------------|---------|----------|------|----------|-------|------|---------|--------|--------|
| github.com/expr-lang/expr | v1.17.8 | 2.0 MB   | 0    | 2.0 MB   | 1     | 1    | 2       | direct |        |
| github.com/go-bridget/mig | v0.6.1  | 107.9 KB | 30   | 57.8 MB  | 4     | 4    | 2       | direct |        |
| github.com/jmoiron/sqlx   | v1.4.0  | 64.6 KB  | 3    | 449.0 KB | 12    | 8    | 2       | direct |        |
| modernc.org/sqlite        | v1.57.0 | 22.7 MB  | 13   | 54.4 MB  | 3     | 3    | 0       | direct |        |

What each column counts:

| Column    | What it counts                                              |
|-----------|-------------------------------------------------------------|
| `Size`    | the module zip, from the proxy                              |
| `Deps`    | modules of this build the dependency requires, transitively |
| `Total`   | `Size` plus the size of those modules                       |
| `Files`   | files importing the module                                  |
| `Pkgs`    | packages those files belong to                              |
| `Symbols` | distinct symbols reached through it                         |

`Deps` and `Total` are walked over the modules this build carries. A module
required by a dependency and resolved away by this build is not counted, and a
module two dependencies both require counts for both: each row is what that one
dependency brings, not a division of the total.

A dependency with zero symbols and one file is imported for what it registers,
which is how a database driver is imported, so `thin` does not report it.

### What go.sum records

go.sum carries a hash for every version the module graph offered, not the one
version per module the build selected. A version whose source is hashed is
downloaded; a version recorded by its go.mod alone was read for its
requirements and passed over. Where more than one version of a module is
recorded, `-stats` prints a second table.

| Module                      | Versions | Linked               | Size     | Overhead |
|-----------------------------|----------|----------------------|----------|----------|
| github.com/stretchr/testify | 4        | v1.12.1              | 203.6 KB | -        |
| modernc.org/gc              | 2        | v2 v2.6.5, v3 v3.1.5 | 518.2 KB | 65.8 KB  |

9 modules recorded at more than one version, 1 linked more than once, 65.8 KB
of it linked twice or more.

The two majors of a module are one row: they are two module paths and one
library, and a build requiring both downloads both. `Versions` counts every
version go.sum records of any of them, `Linked` names the ones with a hash of
their source, `Size` adds those up, and `Overhead` is every linked copy past
the largest.

## selfcontained

A declaration that reaches only the imports of its own file and what is
declared beside it in that file is extractable: everything it is built from is
in one place, which is what `go build one_file.go` asks for. One that reaches a
name declared in another file of the package is not, and the file it is in does
not move without the other file.

The measure reads `Declaration.Globals`, which is what a parse recorded of the
names a declaration reached that its own file neither declares nor binds. A
name the package declares in another file is a reach; a name the package does
not declare at all is a local the parse did not see bound, and it counts as
nothing. That is the fuzz in the measure, and it is why the two parsers do not
give quite the same figure: over oida they report 170 and 168 coupled symbols
of 496.

A package of one file is left out, along with generated files. Test files are
counted apart in a column of their own, because a test reaches what it tests
and counting the two together reports every package as more coupled than it is.

```
| Package                                       | Files | Types | Types(s) | Funcs | Funcs(s) | Coupling | Tests  |
|-----------------------------------------------|-------|-------|----------|-------|----------|----------|--------|
| github.com/titpetric/tools/splint/gomod        | 3     | 1     | 1        | 13    | 12       | 7.1%     | 11.8%  |
| github.com/titpetric/tools/splint/model        | 25    | 26    | 14       | 75    | 50       | 34.9%    | 94.4%  |
| github.com/titpetric/tools/splint/schema       | 5     | 3     | 3        | 24    | 13       | 40.7%    | 0.0%   |
| github.com/titpetric/tools/splint/simpleparser | 14    | 9     | 9        | 112   | 68       | 35.8%    | 100.0% |

23 packages of two or more files, 117 files, 740 symbols, 217 reaching another file, 29.3%.
```

`Types(s)` and `Funcs(s)` are the ones reaching nothing outside their file.
`Coupling` is the share of every symbol of the package, vars and consts
included, that reaches another file, so a package of one type used everywhere
beside it reads high.

The second table is the one to act on: the files reaching furthest into the
rest of their package, counted rather than shared, so a file of two symbols
does not head it for having both of them coupled.

```
| File                              | Symbols | Coupled | Coupling | Test |
|-----------------------------------|---------|---------|----------|------|
| linters/funcargs/funcargs_test.go | 13      | 13      | 100.0%   | yes  |
| schema/converter.go               | 9       | 9       | 100.0%   |      |
| simpleparser/scan.go              | 12      | 9       | 75.0%    |      |
| linters/modcheck/result.go        | 18      | 8       | 44.4%    |      |
```

## Statistics

Every linter reports what it measured as well as what it found, because a check
that counts what it looked at has the count in hand by the time it knows what
to report. `-stats` prints those and nothing else, one table per linter, a
blank line apart, each with a line above saying what it is and a line below
summarising it. What `godoc` measured over the fixture:

Documentation of every exported symbol, by package.

| Package                     | Exported | Documented | Share  | Missing | Format | Verbose |
|-----------------------------|----------|------------|--------|---------|--------|---------|
| example.com/fixture         | 5        | 2          | 40.0%  | 1       | 2      | 0       |
| example.com/fixture/handler | 2        | 2          | 100.0% | 0       | 0      | 0       |

14 of 18 exported symbols documented, 77.8%, 4 issues.

A linter picks its own columns. The numbers behind them are
`model.LintMetrics`, keyed by package or by file, holding the linter's own
metric type: `filecheck` measures files and everything else measures packages.

## Schemas

`--schema` writes the types of a tree as a JSON Schema draft-07 document rather
than linting it. It reads a source tree or a document already written for one:

```shell
splint --schema ./... > schema.json
splint --schema --input go-fsck.json > schema.json
splint --schema --strip-prefix example.com/ ./...
```

This was `go-fsck jsonschema`. It renders every package of a tree rather than
the first one it was handed, and a type name two packages both declare is
qualified with the package, since a schema has one namespace and two `Config`
types are not the same type. An interface and a test package describe no shape
and are left out.

## Development

`atkins` formats, tests and installs the command. `task` runs the same three,
then `splint ./...` over this tree. `task parity` prints how far the two
parsers are apart, value by value, and `task bench` how far apart they are in
time.

`SPLINT_WORKSPACE` points the harness at where the projects it reads are
checked out. A project that is not there is skipped, so it reads whatever the
machine has.
