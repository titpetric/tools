# splint

A data model of Go source, two parsers that fill it, and a linting framework over the top.

The model is schema and nothing else. It imports no third party package, so a linter written against it links neither `go/ast` nor `x/tools`. A parser fills the model and a linter reads it; neither refers to the other.

The command runs in a pipeline. It exits on a code a job branches on, and it renders the one report three ways for the three readers a check has: the pipeline that gates on it, the operator watching a terminal, and whoever reads the artifact afterwards. Where the output goes decides which rendering a run produces, so the same command line serves all three.

## What it obsoletes

Two earlier tools of ours:

- [go-fsck](https://github.com/titpetric/exp/tree/main/cmd/go-fsck) - the extract/lint/docs/coverage/diff toolchain over the same document model. Every subcommand lives on here, and a document it wrote still loads.
- [gofsck](https://github.com/titpetric/tools/tree/main/gofsck) - the single-pass package layout analyser. Its six checks (pairing, coverage, grouping, wraphandler, filecheck, visibility) run here as linters.
- [go-ddd-stats](https://github.com/titpetric/exp/tree/main/cmd/go-ddd-stats) - the per file size report. `splint sizes` writes the same JSON and the same d2 histogram off the document, so one walk of the tree serves both.

And the common tools it stands in for:

| Tool                | What splint does instead                                                                                                                                      |
|---------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `goimports`         | `splint fix` adds the missing imports and drops the unused ones, resolved against the tree, not a global index                                                |
| `goimports-reviser` | the same pass writes the block in the configured group order                                                                                                  |
| `gofumpt`           | nothing: gofumpt, a stricter gofmt, rarely rewrites anything, and gofmt already runs. Formatting rules under the same philosophy may land here later |
| `golangci-lint`     | the twelve linters here, a smaller and more opinionated set                                                                                                   |

## Install

```bash
go install github.com/titpetric/tools/splint@main
```

## Use

```bash
splint ./...                          # lint everything below here
splint fix ./...                      # rewrite every import block, and report nothing
splint --fix ./...                    # rewrite them, then report what is left
splint docs ./... > docs/api.md       # render the tree as an API reference
splint coverage --input splint.json   # report the coverage a document carries
splint diff --old a.json --new b.json # what a release takes away
splint sizes --input splint.json       # every file with its byte size, as JSON
splint --save ./...                   # extract a document to splint.json, and judge nothing
splint -i ../oida ./...               # lint another tree
splint --parser=simpleparser ./...    # read it without building a syntax tree
splint --linters godoc,imports ./...  # run two of the twelve
splint --output model.json ./...      # keep the document under another name
splint --input model.json             # lint a document read back from a file
splint --include-tests --save ./...   # keep the test packages in what is written
splint ./... --json                   # the findings as JSON, no rendering
splint ./... --yaml                   # the same, as YAML
splint --schema ./...                 # write the tree as a JSON Schema
splint --stats ./...                   # what the linters measured, not what they found
splint --offline ./...                # never ask the module proxy
```

A flag reads the same before the pattern or after it. The exit code is 0 for a clean run, 1 when a linter reported something, and 2 when the run itself failed, so a pipeline can tell a finding from a failure.

`--linters none` runs no linter at all, which is the extract: splint parses the tree and writes the document, and the run exits 0 whatever the tree looks like. A job that needs only the document calls this, in place of suffixing a lint with `|| true`.

`--save` writes the parsed document to `splint.json`, beside the tree it describes, and runs no linter: a run given `--save` is there for the document it writes, so the exit code says nothing about the tree. `--output` names another file. splint resolves both against the directory the command ran in, whichever tree `-i` pointed the parse at: the parse moves the process into that tree and puts it back when it finishes.

Every run parses the tree. `--input` is the one way to read a document a previous run wrote, and it makes a second run quick. Over oida:

| Run                      | Time  |
|--------------------------|-------|
| parsed                   | 637ms |
| read back with `--input` | 12ms  |

A `splint.json` found beside the tree never stands in for the parse, so a job that extracts twice gets the second document, and the second run reports the tree it read.

`--include-tests` decides what the written document keeps. Every parse reads the test files, because `pairing` and `coverage` have nothing to check without them, and a document written without the flag carries no test package and no declaration from a `_test.go` file. A report rendered off one listed `cmd/lessgo_test` as a package otherwise.

`splint --help` prints a page in place of a list of flag defaults: what the tool is, how to call it, every flag with its default, and runs worth copying. A terminal gets it in colour and anything else gets it as markdown; `splint --help > docs/splint.md` writes that markdown.

## splint fix

`splint fix ./...` rewrites the import block of every file that does not hold the one the rules describe, and formats every file it reads. `splint --fix ./...` does the same and then lints the tree the rewrite left, so nothing it cleared is in the report.

The formatting is gofmt's, applied through `go/format` after the fixer splices the block in. The fixer formats because nothing else in the workspace runs gofmt - the shared `atkins go:fmt` task is this command - and because a file whose block is already right still gets its struct fields aligned, where skipping it would leave them as typed. A run says which files it rewrote and which it only formatted, and the `imports.fixed` counter still counts the first. A file that does not parse keeps its block rewrite and skips formatting: a syntax error is the compiler's finding, not this command's. The fixer leaves a generated file alone down to the bytes.

The fixer reads the tree with the simple parser unless `--parser` names one. A file that is missing an import it needs does not compile, and that is a file the fixer is there to repair; the ast parser resolves a tree through the toolchain, so its reading is the one least likely to survive such a file. Reading the text also costs ten times less over the same tree, and an editor runs a formatter on every save.

Either parser answers the same. The ast parser reads the import declarations and the generated marker off the file's own bytes, because the package loader returns a cgo file in a form its author never wrote: the toolchain preprocesses it first, so `import "C"` disappears, an import of `"unsafe"` takes its place, cgo's own generated header sits on top, and the line numbers belong to a file that only exists inside the build.

A file that imports nothing is a different thing and is valid Go. Nothing here touches one.

### The groups

The fixer writes an import block as one parenthesised declaration, the groups in this order, sorted by path within each group and separated by a blank line:

| Group     | What is in it                                 |
|-----------|-----------------------------------------------|
| `std`     | a path whose first segment carries no dot     |
| `blanked` | an import written `_` for its side effect     |
| `general` | a third party dependency                      |
| `company` | a path under a prefix `imports.company` names |
| `project` | a path under the module the tree builds       |
| `dotted`  | an import written `.`                         |

That order is the default, `std,blanked,general,company,project`. `imports.order` in `.splint.yml` changes it, and a group the list leaves out follows the ones it names; the fixer drops none.

The fixer takes off an alias repeating the last segment of the path, because the path already says it. A path ending in a major version gets one, because the path does not: `github.com/go-pg/pg/v9` becomes `pg "github.com/go-pg/pg/v9"`, and `gopkg.in/yaml.v3` needs none because the segment already reads `yaml`. A path that implies no name gets none: `example.com/v2` without the version reads `example.com`, which no import can take as its name. An alias somebody chose stays, in the group its path belongs to.

Two import declarations in one file merge into one. A comment above an import and a comment beside it both survive the rewrite.

### Resolving a name

`model.User` in a file that imports no model is the ambiguous case: a global index answers with whichever model it indexed first, so a tree where `/service1` and `/service2` each hold one gets the wrong import half the time.

Resolution here is source relative. The walk starts at the directory holding the name and goes up, stopping at the module the writing package belongs to. From `service2/storage`, `model` is `service2/model`. From `service1/handler` the same spelling is `service1/model`.

The fixer consults four sources, in this order, and the first that answers wins:

| Source   | Meaning                                                           |
|----------|-------------------------------------------------------------------|
| `alias`  | an entry in `imports.aliases` of `.splint.yml`                    |
| `local`  | a package of the tree, found by walking up from the directory     |
| `tree`   | a path some other file of the tree already imports under the name |
| `module` | a go.mod `require` entry whose last segment is the name           |

`tree` is what places a dependency with no module cache read and nothing asked of the network: one file spelling out `github.com/stretchr/testify/assert` teaches every other file in the tree what `assert` means. `alias` is there for the name the tree cannot place on its own, and it wins outright.

A name two paths answer to stays unresolved. A formatter guessing between them writes the wrong import half the time, so the fixer reports the file under `imports/unresolved` and leaves it untouched.

### What it does not do

- It does not write `go.mod`. Removing the last import of a dependency leaves its `require` line in place until `go mod tidy` takes it out.
- It does not touch a file carrying the `Code generated ... DO NOT EDIT.` marker.
- It does not move the `import "C"` of a cgo file. The comment above the import holds the C source cgo compiles, and that comment counts as the preamble only while it sits directly above that import, so the fixer pins that one declaration in place. The fixer formats every other block in the file as usual.
- It does not touch a file carrying a `//line` directive. The directive rebases the line numbers the compiler reports from that point on, so moving a physical line of such a file breaks every position the compiler reports after it. They are machine output, from goyacc and its like.
- It does not touch a file behind a build constraint this build does not meet. Linux skips a `//go:build windows` file, so the fixer neither formats nor reports it there.
- It does not touch anything outside the import declaration. Indentation and the rest of the file are `gofmt`'s.
- It does not remove an import whose name it does not know. Code reaches `github.com/goccy/go-yaml` as `yaml`, and the last segment of the path is not a Go identifier, so the name is a guess. An import removed on a wrong guess leaves a file that fails to build; one kept on a wrong guess is a line nobody notices.
- It does not add an import of the package the file is part of. A file writing `package whitebox` reaches `whitebox.Bar` by a name it already has, and importing it would be a cycle. A file writing `package whitebox_test` is a package of its own and has to import the one it tests, so that one gets the import.
- It does not lose a line. A comment above an import, beside it, or belonging to no import at all comes back in the rewritten block. The line endings of the file and its lack of a trailing newline come back too.

Before writing anything, the fixer checks that the lines up for replacement hold an import declaration. The range comes from a parse, and a parse that read it wrong would otherwise have the rewrite overwrite whatever is really there.

### Configuration

Two files, and they belong to different people.

| File                                                | Holds                                  | Written by                          |
|-----------------------------------------------------|----------------------------------------|-------------------------------------|
| `.splint.yml` beside `go.mod`                       | the rules the fixer formats a tree by  | its authors, by hand, and committed |
| `splint.yml` under the user configuration directory | what runs on this machine have counted | `splint fix`                        |

`.splint.yml` sits at the root of the tree. The fixer formats a tree with no such file by the defaults.

```yaml
imports:
  order: [std, blanked, general, company, project, dotted]
  company:
    - github.com/titpetric/
  project: github.com/titpetric/tools/splint  # defaults to the module path
  set-alias: true
  aliases:
    clone: github.com/huandu/go-clone
  pollution:
    per-package: 2
    file-share: 0.5
    include-tests: true
```

The file states rules and nothing else. What a run counted stays on the machine, in `splint.yml` under the directory the operating system keeps a user's configuration in, which is `~/.config/splint.yml` on Linux:

```yaml
imports:
  fixed: 0
```

`imports.fixed` is how many times a `.go` file has had its import block rewritten, across every run this user has made against any tree. The counter lives outside the tree: a tree's own file states what its authors decided and commit, and a counter records what one person's runs have done. Keeping it in the tree would leave every run that rewrote a file with a dirty repository and every branch carrying a different number.

`splint fix` adds to the counter and writes the file back through its node tree, so a comment written beside one survives it. A run that rewrote nothing does not write at all.

## Output

The reader decides what splint writes.

A terminal is a person. They get what each linter found, and then the findings:

```
98 issues from 12 linters.
╭───────────┬───────┬──────┬──────┬───────┬──────────────╮
│ Linter    │ Error │ Warn │ Info │ Total │ Rules        │
├───────────┼───────┼──────┼──────┼───────┼──────────────┤
│ func-args │       │ 1    │      │ 1     │ order 1      │
│ pairing   │       │ 31   │      │ 31    │ unpaired 31  │
│ coverage  │       │ 64   │      │ 64    │ uncovered 64 │
│ filecheck │       │ 1    │      │ 1     │ long 1       │
│ modcheck  │       │      │ 1    │ 1     │ thin 1       │
╰───────────┴───────┴──────┴──────┴───────┴──────────────╯
godoc, imports, func-returns, grouping, wraphandler, visibility, selfcontained found nothing.

WARN: db/connect.go: pairing/unpaired: connect.go has no connect_test.go beside it
WARN: db/connect.go:19: coverage/uncovered: Connect - exported symbol has no test named TestConnect
```

A tree of any size answers with more findings than a screen holds, so which linter is talking and about what comes first. `ERROR` prints in red, `WARN` in amber and `INFO` in teal, the position in teal, the rule in grey and the symbol in violet; the message carries no colour, because a reader stops on that line. A linter that ran and found nothing gets its name listed under the table and no row of zeroes.

Anything else is a program, and the program that reads a log is GitHub Actions. It gets one [workflow command](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands) per finding, and each command puts its finding on the file and the line of a pull request review:

```
::warning file=db/connect.go,line=19,title=coverage/uncovered::Connect - exported symbol has no test named TestConnect
::error file=go.mod,title=modcheck/replace::example.com/two - go.mod replaces example.com/two with ../local/two
::notice file=go.mod,title=modcheck/thin::github.com/lib/pq - github.com/lib/pq is reached from one file through one symbol
```

`ERROR` is an `::error`, `WARN` a `::warning` and everything below it a `::notice`, which is the three GitHub has. A rule reporting a block of lines sets `Position.EndLine` and the command carries `endLine`, so the annotation spans the whole block, from the line it opens on to `endLine`. The message escapes `%`, CR and LF, and a property escapes those and `:` and `,`, the same as `@actions/core`.

Nothing else in a log becomes an annotation on its own: a compiler shaped line does it only where a [problem matcher](https://github.com/actions/toolkit/blob/main/docs/problem-matchers.md) matches it. GitHub shows ten warnings and ten errors per step and fifty annotations per job and drops the rest silently, so a gate reads the exit code and ignores the count of annotations.

`--json` writes the findings as data, the same run with the rendering skipped, and `--yaml` writes the same thing in the format a document uses. One data model serves both: every field carries a json and a yaml tag naming the same key, and a severity is a text marshaller, which both encoders read, so either one prints the word.

```json
{
  "Issues": [
    {
      "Linter": "godoc",
      "Rule": "missing",
      "Severity": "WARN",
      "Position": { "Package": "model", "File": "model/trace.go", "Line": 7 },
      "Symbol": "Trace",
      "Message": "exported symbol lacks a godoc comment"
    }
  ],
  "Linters": ["godoc", "pairing"]
}
```

`--stats` writes tables, because what it writes is numbers. A terminal gets them drawn and anything else gets them as markdown, padded the way `mdox fmt` pads one so the next documentation build leaves a document holding one untouched. `-stats --json` and `-stats --yaml` write what each linter measured: the metrics, keyed by package or by file, and the tables it would have drawn.

## The two parsers

A program constructs either one with `New(splint.Options)` and both return a `*model.DocumentRoot`, so an import picks which one a program uses.

|                              | `astparser`                    | `simpleparser`               |
|------------------------------|--------------------------------|------------------------------|
| Reads through                | `go/ast` and `x/tools`         | bytes                        |
| Exact                        | yes, the toolchain resolves it | to the margin measured below |
| Source that does not compile | depends on the toolchain       | reads it regardless          |
| Default                      | yes                            | on request                   |

`simpleparser` finds a declaration by where it starts and where it ends. gofmt puts every top level declaration at column zero, and the brace or paren that closes one at column zero as well, so the scanner finds a function's extent without balancing a brace inside it. That extent, taken verbatim, is the source the model records.

A parser that builds no tree cannot fill a schema that names one, so the model carries no `go/ast` types. Nothing in the model is Go specific.

### Speed

`task bench` times both parsers by running the command over every project, which measures what a caller of the command experiences; a benchmark inside the process measures something else.

```
project           files  astparser     simple    ratio
cli                   6      131ms        5ms    25.5x
oida                132      671ms       38ms    17.7x
atkins              276     2.523s      112ms    22.5x
phpscript           319     5.573s      138ms    40.5x
total                      18.999s      697ms    27.3x
```

Sixteen repositories, 1,843 Go files; the `total` row above holds the time for each parser. The ratio rises with the size of the tree. The ast parser resolves a package against everything it imports; a line scan does not.

The timing runs one linter. `modcheck` queries the module proxy about every dependency, a round trip per module that takes longer than either parser.

### Profile

`task profile` takes ten seconds of parsing with a cpu and a memory profile and prints the top of both. What it reported, in the order of what each cost:

| Fixed                                                                          | Cost                         |
|--------------------------------------------------------------------------------|------------------------------|
| Counting the branch keywords with one pass over the line per keyword           | 24% of cpu                   |
| The build constraint splitting the whole file to read a line from its header   | 10% of allocations           |
| `StringSet.Map` compiling its version regexp once per import                   | 7% of allocations            |
| `strip` copying every line out and back, and most lines have nothing to blank  | 12% of allocations           |
| The line facts held as four parallel slices                                    | one allocation per file each |
| Joining every signature into a new string, and most are one line already       | not measured separately      |
| `declaredNames` parsing every signature to read the name off the front of it   | not measured separately      |
| `selectors` returning a slice per line of every body, and the caller kept none | not measured separately      |

Together, on the same tree with the same output:

| Measure     | Before         | After          |
|-------------|----------------|----------------|
| time        | 76ms           | 45ms           |
| memory      | 22.1MB         | 15.3MB         |
| allocations | 225 thousand   | 140 thousand   | The parity harness ran unchanged over the same repositories.

### Parity

`task parity` runs both parsers over sixteen repositories and compares what they produced for the same tree, value by value, with the ast parser as the reference.

The comparison walks the encoded documents value by value and checks no fixed list of fields: the walk visits every key either side carries, a key only one of them has is a difference, and the walk covers a key added to the model later without an edit here. The harness keys the declaration lists on the symbol before the walk, so one extra declaration counts as one difference and never shifts the lists so that every declaration after it misreports.

```
total: 270807 values compared, 1677 differ (0.6193%)
  Funcs[].Complexity.Cognitive   1563  gocognit weights a branch by how deeply it nests
  Vars[].Source                     6  unexplained
      ./server/model Vars[setting.go.byteUnits].Source: // byteUnits are the suffixes [...]
  ...
unexplained: 101 of 270807 values, 0.0373%, budget 0.1000%
```

Cognitive complexity accounts for 1563 of the 1677: `gocognit` weights a branch by how deeply it nests in the syntax tree, and a line scan has no tree to read the nesting from. What remains is 101 values in 270,807, and the harness asserts that as a budget: a change that pushes it higher fails the test. Each difference prints an example.

Three differences are representational, and the harness accounts for them without counting them:

- **Column alignment.** The ast parser renders a declaration through `go/printer`, which writes the padding that lines up a run of struct fields as tabs. The file on disk carries what gofmt wrote, which is spaces. The harness collapses the padding inside a line before comparing the two.
- **The globals and the references.** The ast parser resolves a name against the type-checked scope and the line scanner decides from what the file declares and what the body binds, so the two part company on a few percent of the names. A reader of either field resolves the names against what the package declares, so the difference does not matter, and the count leaves both out.
- **Inline struct types.** For a field whose type spells out a struct, `go/printer` renders it on one side and the scanner reads it off the file on the other, so the layout differs while the type does not. Whitespace collapses before the comparison.

## Packages

| Package                 | What                                                                                                 |
|-------------------------|------------------------------------------------------------------------------------------------------|
| `model/`                | the schema, and the linter interfaces over it. No third party imports                                |
| `parsers/analyzer/`     | the `go/ast` parser                                                                                  |
| `parsers/simpleparser/` | the parser that reads bytes                                                                          |
| `gomod/`                | reads a go.mod and a go.sum into the model, and catalogues what they require                         |
| `modproxy/`             | queries the Go module proxy for what a dependency weighs, its age and what it requires               |
| `model/loader/`         | reads a document back from `.json` or `.yml`                                                         |
| `coverprofile/`         | folds a Go coverage profile into a parsed document                                                   |
| `refindex/`             | the reverse lookup: which declarations reach a symbol, read off the references                       |
| `filescope/`            | what a file needs from the rest of its package, and whether it needs nothing                         |
| `commands/coverage/`    | renders the coverage a document carries as tables, or through a template                             |
| `commands/diff/`        | compares two documents: the exported API and the go.mod behind it                                    |
| `commands/docs/`        | renders a document as an API reference: markdown, spec, imports, plantuml, or one symbol godoc style |
| `linters/`              | the registry, one subpackage per linter                                                              |
| `importfmt/`            | the house rule for an import block, as a function of the imports and nothing else                    |
| `resolve/`              | what package a bare name refers to, from the directory that holds it                                 |
| `commands/fix/`         | rewrites an import block. The only package here that writes a file                                   |
| `config/`               | reads the tree's `.splint.yml`, and keeps the run counter under the user configuration directory     |
| `schema/`               | renders a document as a JSON Schema                                                                  |
| `report/`               | the findings: the issues, sorted and counted                                                         |
| `render/`               | how it looks: the issue line, drawn tables, markdown                                                 |
| `pkg/splint/`           | Options and the Parser interface, what a program constructs the parsers from                         |
| `tests/`                | the parity harness and the benchmark                                                                 |

The dependency runs one way. `model` imports nothing of its own; `pkg/splint` imports `model`; the parsers import both; the linters import `model` alone; the command at the module root imports everything, so the root is `package main` and the install line is one segment shorter.

`report` holds the findings and `render` holds how they look, so there is one package to open when the output is wrong. The drawn tables are worktree's, ported and not imported: worktree draws them in `package main`, and splint does not depend on a CLI tool's module.

## The model

`model.DocumentRoot` holds what one parse produces: the packages it found and the modules they belong to. Every field carries a `json` and a `yaml` tag naming the same key, so the two encodings describe the same document and `loader` reads either.

The JSON keys are the Go field names. `loader` reads a document back whether it carries the root object or is a bare list of packages, so documents written before the root existed still load.

Nothing in the package marshals anything itself. What it carries is utilities over the data: `DeclarationList.Exported`, `Definition.Merge`, `StringSet.Add`, `Declaration.Position`. Reach for a type with helpers hanging off it; a type with a `MarshalJSON` is not.

### The imports of a file

`Definition.Imports` is a `StringSet` of import literals keyed by filename. A read returns it sorted, the parse drops an alias equal to the last segment of the path, and the set records no line. A rule about what a file imports reads it; a rule about how the file writes them cannot.

`model.File` carries the second view. `ImportDecls` is the declarations as the file writes them, in order, each with its lines, and each spec with its alias, its doc comment, the comment beside it, and whether a blank line separates it from the spec before it. `ImportDecl.Line` and `EndLine` are the lines a rewrite replaces.

`File.Uses` is every identifier the file writes before a dot, sorted. It over-collects: a local variable, a parameter and a receiver are all in it, so `t` from `t.Run` is a use. Over-collecting keeps an import that is not needed. Under-collecting removes one that is, and that breaks the build.

`File.Package` is the name in the file's own package clause, which can differ from the name of the package the model records the file under. A directory holds one package and up to two test scopes, and the external one declares `<name>_test`; the two are one definition in the model and only the clause tells a file of one from a file of the other.

The parsers fill both only when `splint.Options.IncludeImports` holds true. The command sets it on every run; the parity harness does not, so the document it compares stays the same.

## Writing a linter

A linter keeps its own result type and implements two methods on the slice of them. The framework ranges over that slice in place and materialises only the `model.Issue` view of each, so the framework walks a rule carrying twenty fields per finding without copying any of them.

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

Add it to `linters.All()` and it runs. A linter never touches disk and never parses anything: everything it needs is in the document, so the same rule runs over a tree either parser read and over one loaded from a file.

A linter does not read `Declaration.Source` either. A rule is a question about the model, and a rule that has to read the text is a field the model is missing: add the field to the parsers, both of them, and read that. The source is in the document for consumers that are not linters, a browser or an analysis that walks branches, and `splint ./...` does not keep it.

An issue carries an `slog.Level` for its severity, a position that reads `package: file.go:line`, the linter and rule it came from, and an `Attrs` map for anything else the rule has to say. The map is what keeps the schema from growing a field per rule.

`Statistics` is one table: labels, rows, and two options for the line above and the line below.

```go
model.NewStatistics(
	[]string{"Package", "Exported", "Documented"},
	rows,
	model.HeaderText("Documentation of every exported symbol, by package."),
	model.FooterText("14 of 18 exported symbols documented, 77.8%, 4 issues."),
)
```

A linter that measures nothing returns the zero `LintMetrics` and no tables, and the renderer leaves it out; no empty box appears under a heading.

## The fixture

`testdata/` is a module of its own holding code written to fail every check: a file with no test beside it, an exported symbol with no doc, a handler with no wrapper, symbols in files not named for them, two files reaching different modules as `model`, a blank import outside main.go, a file whose every symbol reaches the file beside it, a function taking `(time.Duration, string)` and one returning `(error, *User)`.

Every parser skips `testdata`, `vendor`, `node_modules` and anything opening on a dot or an underscore, so nothing reads it by accident. Point a run at it to read it:

```shell
splint -i testdata ./...
splint --stats -i testdata ./...
```

A walk reads its root whatever the root's name. A walk that skipped the directory passed to it would read nothing at all.

### The formatter fixture

`linters/imports/testdata/` is the second one, made of three trees:

| Directory | What it holds                                                            |
|-----------|--------------------------------------------------------------------------|
| `input/`  | a module written the way people write source no formatter has touched    |
| `golden/` | the same module as the formatter should leave it                         |
| `output/` | what `atkins fix` writes, for reading a change by eye. Gitignored        |

`input/` states each claim about the fixer as source. `main.go` writes two import declarations and an alias that repeats its path. `service1/handler` and `service2/storage` both reach `model.` with no import, and each has to resolve to its own service. `client/client_test.go` reaches `assert.`, and `client/client.go` writing the path out places it. `comments.go` carries a comment above an import and one beside it, and `dangle/` a comment belonging to no import at all. `goyaml.go` imports `github.com/goccy/go-yaml` and writes `yaml`, so the name code reaches the import by differs from the last segment of its path. `odd/` writes its blocks in shapes gofmt would not: an indented closing paren, a block on one line, and a block holding nothing. `whitebox/` and `blackbox/` are the two ways to test a package, and only one of them may import the package it tests. `generated.go` and `cgo/` have to come out unchanged. `broken/broken.go` reaches a name nothing can place.

`TestFixGolden` copies `input/` into a temporary directory, runs the fixer over it with both parsers, and reports every file that does not come out the way `golden/` says. `TestFixIsIdempotent` runs it over `golden/` and asserts that it writes nothing.

```shell
atkins fix   # copy input to output, fix it, and diff it against golden
```

The job ends on that diff, so a change to the formatter shows up as a diff before anyone moves `golden/` to match it.

### The corpus

A fixture holds the cases somebody thought of. The module cache holds the ones nobody did: every dependency this machine has ever downloaded, written by people who never heard of this formatter.

`TestCorpusDoesNotCorruptSource` reads modules out of `GOMODCACHE`, works out what the fixer would write in each, and applies it in memory. The test writes nothing to the cache. It then checks each rewritten file against the source it came from on three things:

- it still parses
- every declaration that is not an import prints exactly as it did before, so a rewrite that deleted a function or truncated a file fails here
- the name the file reaches each import by stays the same, and a path appears or vanishes only where the plan said it would

The sorting keys come from each module's own `go.mod`, so the test formats every module under its own project prefix.

```shell
go test ./tests/ -run TestCorpus                      # 300 modules
SPLINT_CORPUS=0 go test ./tests/ -run TestCorpus      # every module in the cache
```

## The linters

Twelve of them, every one a set of rules over the parsed document.

| Name            | What it reports                                                                                                                                                                                          | What it measures                                                                                                                                            |
|-----------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `godoc`         | an exported symbol with no doc comment, one that does not open on the symbol it documents, one that does not end in punctuation, and one long enough to say the symbol does too much. The linter skips a generated file: its comments are the generator's to write | documented against exported, per package                                                                                                                    |
| `imports`       | five rules, listed below: a name meaning two things, a block not written to the house rule, an import nothing reaches, a name nothing can place, and a dependency that has spread                        | import names and collisions, files formatted, unused and unresolved, per package; and how far every external dependency has reached                         |
| `func-args`     | a two argument function whose arguments read in an order a caller has to look up: a context that is not first, a duration that is not last, two parameters of the same type, an interface after a struct | funcs considered against passing                                                                                                                            |
| `func-returns`  | a function returning an error or a bool before the value it qualifies                                                                                                                                    | funcs considered against passing                                                                                                                            |
| `pairing`       | a file with no test named after it, and a test with no file named after it                                                                                                                               | files, tests, paired and standalone, per package                                                                                                            |
| `coverage`      | an exported symbol with no test named for it                                                                                                                                                             | covered against exported, and the constructors, per package                                                                                                 |
| `grouping`      | an exported symbol in a file not named for it, unless the file compiles by itself                                                                                                                         | symbols passing and symbols left to their own file, per package                                                                                             |
| `wraphandler`   | an exported HTTP handler with no unexported function behind it, so only a server can call it                                                                                                             | handlers wrapped, per package                                                                                                                               |
| `filecheck`     | a file long enough to be doing more than one thing                                                                                                                                                       | line counts and their spread, **per file**                                                                                                                  |
| `visibility`    | nothing                                                                                                                                                                                                  | exported against internal, and the share of a package its private half occupies                                                                             |
| `selfcontained` | nothing                                                                                                                                                                                                  | what a file needs from the rest of its package: symbols, types and funcs reaching nothing outside the file they are in, and the share that do, **per file** |
| `modcheck`      | five module rules, listed below                                                                                                                                                                          | every dependency: size, reach, files, packages, symbols used, kind; and every version go.sum records                                                        |

`func-args` considers a function taking exactly two arguments. The order of one pair is unambiguous; for three or more the expected order is a heuristic sort.

`pairing` reads the two directions at different weights. `unpaired`, a file with no test beside it, is a WARN, and the rule excuses it when a test elsewhere references every exported symbol of the file. `orphan`, a test with no file beside it, is an ERROR: `foo_test.go` names `foo.go`, and a directory holding only the first is a rename or a deletion that stopped halfway. `orphan` excuses a directory whose package name contains `test`, because `fstest`, `httptest` and a repository's own `tests` hold test code and no tests of code; the rule strips the `_test` suffix every external test scope carries before reading the name. It also excuses a directory with a `test` or `tests` segment in its path, the place a tree keeps the black box suites that name no file: the package there holds tests and no code, so every file in it would be an orphan and the rule has nothing to say. The rule reads the path as segments, so the spelling of `attestation/` earns it no excuse.

`grouping` leaves a self-contained file alone: one whose declarations reach nothing declared in another file of the package compiles on its own, so it moves as one unit and the placement of a symbol inside it concerns that file alone. `filescope` answers that question for this linter and for `selfcontained`, which measures the same reach per file. The table counts the symbols left alone under Own file and drops none, so it says how many there are.

`visibility` and `selfcontained` report no issues at all, which the interface allows: an empty report and a table. They report the counts and judge nothing. A parser keeps most of its symbols private and a data model exports most of its own, and a package written as one unit across many files is as legitimate as one split into many.

## imports

Five rules. Four of them are about one file's import block; the fifth is about how far a dependency has reached.

| Rule         | Severity | Fixable | What it reports                                                                                                                               |
|--------------|----------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------|
| `collision`  | WARN     | no      | two files of one package reaching different modules under the same short name, which compiles and reads as though they agree                  |
| `format`     | ERROR    | yes     | an import block that is not the one the house rule describes: the wrong order, more than one declaration, an import to add or one to take out |
| `unused`     | ERROR    | yes     | an import no name in the file reaches                                                                                                         |
| `unresolved` | ERROR    | no      | a name the file reaches that nothing can place, or that two paths answer to                                                                   |
| `pollution`  | WARN     | no      | an external dependency reaching more of the tree than one place                                                                               |

A fixable finding carries a `fix` attribute whose value is `splint fix`, and `splint fix ./...` clears every one of them. `unresolved` is not fixable: there is nothing to write.

`unused` and `unresolved` read different sets, and each errs the way that cannot break a build. `File.Uses` over-collects, so the fixer never removes an import it keeps alive. `Declaration.References` and `Declaration.Globals` exclude locals, parameters and the names the file declares, so a name they report really is unbound. The parsers collect references on function bodies, so `unresolved` stays silent on a package reached only from a struct field; the compiler catches that one.

### Pollution

One file is the clean reading. A dependency reached from one file through one name is one place to change when someone replaces it, and one place a reader has to understand to learn what the tree took on.

The rule counts a package of the tree and never reports it. A package structure exists so every consumer under a model package can import it, and a rule that called that pollution would be a rule against writing one. The rule skips the standard library too.

The rule reports an external dependency when two or more files of one directory import it, or when it reaches half the files of the tree. `per-package`, `file-share` and `include-tests` under `imports.pollution` in `.splint.yml` move those, and a zero turns either rule off.

The count is per directory, so a package and its test half count together, and the rule reports a package whose two test files both reach `testify/assert`. Set `include-tests: false` for a tree where that reads as noise.

`modcheck` counts files, packages and symbols per dependency as well. The two measure different things: modcheck measures what a dependency costs to carry, and `pollution` measures how far one import path has spread.

## Diff

`splint diff` compares two documents: the exported API of every package, and the go.mod behind it. [worktree](https://github.com/titpetric/tools/tree/main/worktree) reads it for its release verdicts, so the JSON field names are a contract.

```shell
splint --linters none --output old.json -i /path/to/v1 ./...
splint --linters none --output new.json ./...
splint diff --old old.json --new new.json
splint diff --old old.json --new new.json --json
```

The diff keys a symbol by import path, receiver and name, so it finds the symbol across revisions whichever file it moved to, and a move between files is no change at all. It compares a signature with its parameter names stripped, because renaming a parameter changes no caller.

Breaking is a removed or changed exported symbol, or a field moved on an exported type; an added symbol is not, an added struct field is not, and an added interface method is, because every implementation is now short one. `--include-unexported` compares the unexported declarations and the internal packages too, reported but never breaking, and a go.mod change is never breaking either. `--include-indirect` keeps the indirect requirements in the go.mod comparison.

The rendering is one line per moved symbol under `-`, `~` and `+` markers, the fields that moved, the go.mod changes, and a summary line; `-v` adds what each symbol reads as, and `--json` writes the result as data.

## modcheck

`modcheck` is the linter that reaches outside the document. A size, a publication date and the requirements of a released version are properties of the artifact and absent from the source, so they come from the Go module proxy. The linter downloads nothing: the size is the `Content-Length` of a `HEAD` on the module zip, the version dates are one small JSON document each, and the requirements are the `.mod` file of that version. A machine with no network reports the columns that come out of the document and leaves the rest blank.

### The size cache

A module version is immutable, so what it weighs is a fact that does not expire. The linter keeps the sizes in `~/.cache/splint/sizes.yml`, under the cache directory the machine names, and writes the file once after its last query, one write per run and none per module:

```yaml
github.com/a-h/templ:
    size: 1961490
    versions:
        v0.3.1020: 1961490
```

`size` is the mean of the versions below it, and the linter answers with it for a version the file does not hold. A module is much the same size from one release to the next, so an answer within a few percent of the truth costs a round trip less than the truth does.

The linter writes the file beside itself and renames it over the old one, and reads the new file back before the rename: a cache that does not parse is one the next run would throw away, and throwing it away here costs nobody anything.

`--offline` queries nobody. The sizes come from the cache alone, and the columns only the proxy can fill, the reach of a dependency and how far behind it lags, stay blank; the run never reports them as zero. One run fills the cache and every run after it can be offline.

```
splint --linters modcheck -stats ./...             # 3.1s cold, 1.8s with the sizes cached
splint --linters modcheck -stats --offline ./...   # 0.6s, asks nobody
```

### The rules

| Rule      | Severity | Reports                                                                                                            |
|-----------|----------|--------------------------------------------------------------------------------------------------------------------|
| `replace` | error    | a replace directive, so what the build resolves to is not what the go.mod requires                                 |
| `unused`  | warn     | a required module no file imports                                                                                  |
| `majors`  | warn     | two majors of one module required together, which link both and whose types do not meet each other's interfaces    |
| `thin`    | info     | a dependency reached from one file through one symbol, and not imported for its side effect anywhere               |
| `blank`   | warn     | a blank import in a file that wires no binary: not a program, not a test                                           |

A blank import runs the package's init and reaches no symbol. Which packages the binary links decides what it does: a driver registers itself with `database/sql`, `net/http/pprof` mounts handlers on the default mux. A program decides that for itself, and so does a test binary, so every file of package main and every test file may carry one. A library carrying one decides it for every consumer, and the rule reports that; the finding names the file holding the import, where the other four rules name the go.mod.

```
::warning file=server/handler.go,title=modcheck/blank::example.com/drivers - example.com/drivers is imported for its side effect from handler.go, which wires no binary: what a program links is decided by the program
```

The rule skips two: a package belonging to the tree under read, and `embed`. Where a project puts its own registrations is its own arrangement, and the compiler requires a blank `embed` to embed into a string or a byte slice.

### What a dependency costs

Size is one column. The others are how far the dependency reaches into the tree and how much of the build list it brings with it. Four rows of one run over etl:

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

The walk behind `Deps` and `Total` covers the modules this build carries. A module required by a dependency and resolved away by this build does not count, and a module two dependencies both require counts for both: each row holds what that one dependency brings, and the rows do not divide the total between them.

Code reaches a symbol by name or as a type, and both count. A call in a body names it as a selector, and a struct field, a parameter, a result or a var's type names it as a type; a package whose whole use of a module is a `*pdo.PDO` field that every method drives its queries through reaches it as surely as a caller does. `Files` counts files and not imports, so one file reaching two packages of one module is one file: one place to change when the module goes.

A file importing a dependency for its side effect imports it for what it registers, the way code imports a database driver. A caller cannot inline a registration, so how little of the rest code reaches by name says nothing and `thin` does not report it.

### What go.sum records

go.sum carries a hash for every version the module graph offered, beyond the one version per module the build selected. The toolchain downloads a version whose source go.sum hashes; for a version go.sum records by its go.mod alone, the toolchain read that go.mod for its `require` lines and passed the version over. Where go.sum records more than one version of a module, `--stats` prints a second table.

| Module                      | Versions | Linked               | Size     | Overhead |
|-----------------------------|----------|----------------------|----------|----------|
| github.com/stretchr/testify | 4        | v1.12.1              | 203.6 KB | -        |
| modernc.org/gc              | 2        | v2 v2.6.5, v3 v3.1.5 | 518.2 KB | 65.8 KB  |

```
9 modules recorded at more than one version, 1 linked more than once, 65.8 KB of it linked twice or more.
```

The two majors of a module are one row: they are two module paths and one library, and a build requiring both downloads both. `Versions` counts every version go.sum records of any of them, `Linked` names the ones with a hash of their source, `Size` adds those up, and `Overhead` is every linked copy past the largest.

## selfcontained

A declaration is extractable when it reaches only the imports of its own file and what that file declares beside it: everything in its build sits in one place, the condition `go build one_file.go` requires. One that reaches a name declared in another file of the package is not, and its file does not move without the other file.

The measure reads `Declaration.Globals`: the names a declaration reached, as a parse recorded them, that its own file neither declares nor binds. A name the package declares in another file is a reach; a name the package does not declare at all is a local the parse did not see bound, and it counts as nothing. That is the fuzz in the measure, and the reason the two parsers report slightly different figures: over oida they report 170 and 168 coupled symbols of 496.

The measure skips a package of one file, along with generated files. Test files count apart in a column of their own, because a test reaches what it tests and counting the two together reports every package as more coupled than it stands.

```
| Package                                       | Files | Types | Types(s) | Funcs | Funcs(s) | Coupling | Tests  |
|-----------------------------------------------|-------|-------|----------|-------|----------|----------|--------|
| github.com/titpetric/tools/splint/gomod        | 3     | 1     | 1        | 13    | 12       | 7.1%     | 11.8%  |
| github.com/titpetric/tools/splint/model        | 25    | 26    | 14       | 75    | 50       | 34.9%    | 94.4%  |
| github.com/titpetric/tools/splint/schema       | 5     | 3     | 3        | 24    | 13       | 40.7%    | 0.0%   |
| github.com/titpetric/tools/splint/simpleparser | 14    | 9     | 9        | 112   | 68       | 35.8%    | 100.0% |

23 packages of two or more files, 117 files, 740 symbols, 217 reaching another file, 29.3%.
```

`Types(s)` and `Funcs(s)` are the ones reaching nothing outside their file. `Coupling` is the share of every symbol of the package, vars and consts included, that reaches another file, so a package of one type used everywhere beside it reads high.

The second table is the one to act on: the files reaching furthest into the rest of their package, ranked by count and not by share, so a file of two symbols does not head it for having both of them coupled.

```
| File                              | Symbols | Coupled | Coupling | Test |
|-----------------------------------|---------|---------|----------|------|
| linters/funcargs/funcargs_test.go | 13      | 13      | 100.0%   | yes  |
| schema/converter.go               | 9       | 9       | 100.0%   |      |
| simpleparser/scan.go              | 12      | 9       | 75.0%    |      |
| linters/modcheck/result.go        | 18      | 8       | 44.4%    |      |
```

## Statistics

Every linter reports what it measured as well as what it found, because a check that counts what it looked at has the count in hand by the time it settles what to report. `--stats` prints those and nothing else, one table per linter, a blank line apart, each with a line above saying what it holds and a line below summarising it. What `godoc` measured over the fixture:

Documentation of every exported symbol, by package.

| Package                     | Exported | Documented | Share  | Missing | Format | Verbose |
|-----------------------------|----------|------------|--------|---------|--------|---------|
| example.com/fixture         | 5        | 2          | 40.0%  | 1       | 2      | 0       |
| example.com/fixture/handler | 2        | 2          | 100.0% | 0       | 0      | 0       |

14 of 18 exported symbols documented, 77.8%, 4 issues.

A linter picks its own columns. The numbers behind them are `model.LintMetrics`, keyed by package or by file, holding the linter's own metric type: `filecheck` measures files and everything else measures packages.

## Coverage

`--append-coverage=FILE` reads a Go coverage profile, the file `go test -coverprofile` writes, and folds what it says into the document the parse produced:

```bash
go test -count=1 -cover -coverpkg=./... -coverprofile=pkg.cov ./...
splint --save --append-coverage=pkg.cov ./...
```

The fold fills `model.Complexity.Coverage` on every function and method, from the profile blocks that fall inside the file and the line range the parse recorded for it, and on every package, over the declarations in it. Both weigh by statement and not by block, the same as `go tool cover -func` reports: covered statements over total statements. A declaration with no statements reports 0, matching the cover tool.

At the same time, a package gets the complexity of its declarations added up. Nothing else fills `model.Package.Complexity`, and a report printing coverage beside complexity reads both off the same struct.

The flag belongs to a parse. `--input` reads a document a previous run extracted, and the command refuses the two together; it never silently does one of them.

The package figure weights every function by the statements behind it, so a package of one long uncovered function and ten short covered ones reads low.

`splint coverage` renders what the fold added: one markdown row per covered function with its cognitive complexity beside the coverage, and one per package with the lines added up. `-v` keeps the uncovered functions in, `--json` writes the rows as data, and `--template` renders a text/template with both tables, `.Functions` and `.Packages`, in place of printing the function table alone.

```bash
go test -count=1 -cover -coverpkg=./... -coverprofile=pkg.cov ./...
splint coverage --append-coverage=pkg.cov ./... > docs/testing-coverage.md
splint coverage --input splint.json --template docs/testing-coverage.md.tpl
```

The report reads the same two fields whichever way the document arrived: a parse with the profile folded in, or `--input` naming a document written with one.

## Sizes

`splint sizes` reports what the document weighs: every file with its byte size, the per directory totals, and a histogram over the files in powers of two:

| Histogram bucket | Size   |
|------------------|--------|
| smallest         | 1 KB   |
| largest          | 256 KB |

The output is JSON, in the shape `go-ddd-stats` wrote, so a consumer storing these payloads reads old and new rows the same way.

```shell
splint sizes ./...
splint sizes --input splint.json
splint sizes --render d2 ./... | d2 --layout elk - docs/assets/size.svg
```

`--render d2` writes the histogram as a d2 source document, one rectangle per non-empty bucket; the size diagram in a README renders from it.

The command counts every file the document carries, whatever scope declared it: a document written with `--include-tests` counts the test files, one written without does not.

## Schemas

`--schema` writes the types of a tree as a JSON Schema draft-07 document and lints nothing. It reads a source tree or a document already written for one:

```shell
splint --schema ./... > schema.json
splint --schema --input model.json > schema.json
splint --schema --strip-prefix example.com/ ./...
```

The schema covers every package of the tree, and qualifies a type name two packages both declare with the package, since a schema has one namespace and two `Config` types are not the same type. An interface and a test package describe no shape, so the schema leaves them out.

## Docs

`splint docs` renders the tree as an API reference and lints nothing. The reference is the package godoc, every exported type, const and var behind a `<details>` fold, the signature of every exported function, and the godoc examples printed whole from the test package they live in. The reference prints no function bodies.

```shell
splint docs ./... > docs/api.md
splint docs --input splint.json > docs/api.md
splint docs --render puml ./... | plantuml -tsvg -pipe > docs/structure.svg
splint docs --render imports ./... | plantuml -tsvg -pipe > docs/imports.svg
splint docs --split --out docs/api --strip-prefix github.com/titpetric ./...
```

A parse for the documentation keeps the sources with or without a flag requesting them, the way `--schema` does, so `splint docs -i ./migrate .` is the whole of a documentation job: no document on disk and nothing to clean up. `--render` picks the rendering: markdown is the default, `spec` is the declared symbols alone, `imports` and `puml` are plantuml diagrams of what the packages reach, `config` is a configuration reference, and `json` is the document itself. `--split` writes one markdown file per package under `--out` with a README listing them, named after the import path with `--strip-prefix` taken off the front. `--hide` leaves named types out of the puml diagram and `--model` draws the data model alone.

### The configuration reference

`--root` opens the docs on one type and renders them as a configuration reference: each type under its own heading, each field as a bold line carrying its json name and its type, and the field doc as the paragraph under it. A field whose type the package declares links to that type's section, so a reader walks a nested configuration from the root; the root type prints first and the types it reaches follow in reach order. `--title` replaces the root type's heading.

```shell
splint docs --root Config --title "# Configuration" -i ./server/config . > docs/config.md
```

A `bool` reads as `boolean` and an `interface{}` as `any`, because the reference describes a document and leaves out the Go that decodes it. The reference describes a type with no fields by its underlying type, linked where that is a declared type.

## Development

`atkins` formats, tests and installs the command. `task` runs the same three, then `splint ./...` over this tree. `task parity` prints how far the two parsers are apart, value by value, and `task bench` how far apart they are in time.

`SPLINT_WORKSPACE` points the harness at the checkout directory of the projects it reads. The harness skips a project that is missing, so it reads whatever the machine has.
