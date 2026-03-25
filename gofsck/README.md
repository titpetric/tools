# gofsck - A static analysis tool for Go package structure

A Go filesystem check tool with modular analyzers for package structure validation.

Gofsck provides five independent analyzers that can be run in multiple modes:

1. **Pairing Analyzer** - Validates file-test relationships (e.g., `file.go` with `file_test.go`)
2. **Coverage Analyzer** - Analyzes symbol-test coverage and naming patterns
3. **Grouping Analyzer** - Ensures exported symbols are in appropriate files
4. **Wraphandler Analyzer** - Ensures exported HTTP handlers have corresponding unexported error-returning wrappers
5. **Filecheck Analyzer** - Gauges complexity by file size distribution and cognitive load ratings

## Installation

```bash
go install github.com/titpetric/tools/gofsck@latest
```

## Usage

### Run All Analyzers (Default)

```bash
./gofsck ./...
./gofsck .
```

### Output Formats

**Text output (default):**
```bash
./gofsck -format text ./pkg/mypackage
```

**JSON output:**
```bash
./gofsck -format json ./pkg/mypackage
```

**YAML output:**
```bash
./gofsck -format yaml ./pkg/mypackage
```

### Save Report to File

```bash
./gofsck -format json -output report.json ./...
./gofsck -format text -output report.txt ./...
```

### Linter Integration

For golangci-lint integration, use the grouping analyzer via singlechecker:

```yaml
# .golangci.yml
linters:
  - gofsck
```

See [.golangci.yml](./.golangci.yml) for configuration examples.

## Analyzers

### 1. Pairing Analyzer

Checks that Go source files have corresponding test files.

**Output:**
- `files` - Count of non-test Go files
- `tests` - Count of test Go files
- `paired` - Number of files with matching test files
- `standalone_files` - Non-test files without tests
- `standalone_tests` - Test files without corresponding source files

**Example:**
```json
{
  "files": 42,
  "tests": 40,
  "paired": 38,
  "standalone_files": 4,
  "standalone_tests": 2
}
```

**Package:** `pkg/pairing/`

### 2. Coverage Analyzer

Analyzes symbol-test coverage using naming conventions.

**Test Naming Conventions:**
- Package function `Flatten` -> expects `TestFlatten*`
- Method `Server.Get` -> expects `TestServer_Get*`
- Context suffixes allowed: `TestServer_Get_WithContext`

**Output:**
- `symbols` - Map of exported symbols to their test functions
- `covered` - Number of covered symbols
- `uncovered` - Symbols without test coverage (includes expected test name and test kind)
- `constructors` - Number of constructor functions (`New*`), excluded from uncovered count
- `standalone_tests` - Test functions with no matching symbol
- `coverage_ratio` - Percentage of covered symbols (0.0 to 1.0)
- `adjusted_coverage` - Coverage ratio including constructors as covered
- `want_unit_tests` - Number of uncovered symbols needing unit tests
- `want_integration_tests` - Number of uncovered symbols needing integration tests

**Logic:**
- Walks AST to find exported symbols (functions, types, methods)
- Extracts test function names using naming conventions
- Constructor functions (`New*`) are tracked separately and excluded from uncovered symbols
- Methods are covered implicitly when the receiver type has a test (e.g., `TestServer` covers `Server.Get`)
- Symbols in `/storage` or `/repository` packages are flagged as needing integration tests
- Reports expected test function names for uncovered symbols (e.g., `TestFlatten` for `Flatten`)
- Calculates both standard and adjusted coverage ratios

**Package:** `pkg/coverage/`

### 3. Grouping Analyzer

Ensures exported symbols are grouped in files matching their names. Available as a golangci-lint plugin.

**Matching Rules:**
- Symbol names converted to snake_case
- Singular/plural variations supported
- Base noun extraction (e.g., Runner -> Run)
- Wildcard patterns (e.g., `service*.go`)
- Allowlist for `model*.go`, `types*.go`

**Patterns:**
- Type `ServiceDiscovery` -> `service_discovery.go`, `discovery.go`, or `service.go`
- Method `ServiceDiscovery.Get` -> `get.go`, `discovery_get.go`, or `service_discovery_get.go`

**Package:** `pkg/grouping/`

### 4. Wraphandler Analyzer

Ensures exported `http.HandlerFunc` functions have corresponding unexported wrapper functions that return `error`.

**Convention:**
- Exported handler `Handler(http.ResponseWriter, *http.Request)` -> expects unexported `handler(w, r) error`
- Receiver method `(*Service).Handler(http.ResponseWriter, *http.Request)` -> expects `(*Service).handler(w, r) error`

**Output:**
- `total` - Number of exported HTTP handlers found
- `passing` - Number of handlers with matching unexported wrappers
- `violations` - Handlers missing their unexported wrapper

**Example:**
```json
{
  "total": 50,
  "passing": 15,
  "violations": [
    {
      "file": "handler.go",
      "line": 42,
      "symbol": "Handler",
      "receiver": "Service",
      "message": "Service.Handler implements HandlerFunc, expected (*Service).handler(w, r) error"
    }
  ]
}
```

**Summary:** `15/50 handlers passing`

**Package:** `pkg/wraphandler/`

### 5. Filecheck Analyzer

Gauges codebase complexity by analyzing file size distribution across the module. Based on cognitive load estimates for both humans and LLM agents.

**Output:**
- `scanned` - List of file groups by extension, each with:
  - `ext` - File extension (e.g., `.go`, `.md`)
  - `files` - Number of files
  - `histogram` - File size distribution in power-of-two KB buckets (1, 2, 4, 8, 16, 32...)
  - `score` - Score for the extension group
- `rating` - Overall rating: `(sum of ≥16KB file sizes / sum of all file sizes) * 100`

**Example:**
```json
{
  "scanned": [
    {
      "ext": ".go",
      "files": 105,
      "histogram": [20, 45, 25, 10, 3, 2, 0],
      "score": 95.20
    }
  ],
  "rating": 95.20
}
```

**Note:** Generated code (protobuf `pb.go`, bindata, gRPC service definitions) may penalize the score.

See [pkg/filecheck/README.md](pkg/filecheck/README.md) for cognitive level tables and benchmarks.

**Package:** `pkg/filecheck/`

## Development

There is a Taskfile.yml provided with typical development tasks.

Run `task` to build everything locally.

## Background

Go is a package driven language. The most common architecture method in
the wild is [The Big Ball of Mud](https://blog.codinghorror.com/the-big-ball-of-mud-and-other-architectural-disasters/).

Go supports building and running tests in more fine-grained ways:

- `go build file.go`
- `go test file.go file_test.go`
- `go test file_test.go`

For this to work:

1. `file.go` has to be "single responsibility" (not using other symbols in package)
2. `file.go` and `file_test.go` should have no "globals" in use
3. `file_test.go` has to be a black box test (can be moved anywhere)

This lets engineers reason better about how something is tested. With
large package scopes which are typical for the "big ball of mud"
architecture style, renaming files to correspond to symbol name is
typical practice in some other languages.

## Scope

The tool checks that typed structs and functions with an exported
receiver are grouped into the correct expected files:

- A symbol of `ServiceDiscovery` is expected in `service_discovery.go` or `service*.go`.
- A symbol of `ServiceDiscovery.Get` is expected in `service_discovery_get.go` or `service_discovery.go`, or `service*.go`.

Beyond this assertion, some typical filenames are allowed, based on
symbol type grouping that's a common practice in smaller projects:

- `model*.go` - holds data model `type` definitions
- `types*.go` - holds `type` definitions

Ultimately, a lot of packages are small and may contain a single file.
For a package named `sqlite3`, the exception is to group all symbols in
`sqlite3/sqlite3.go`.

## Future

The structure is checked with unit tests in mind. Black box unit tests are
a good way to have the full symbol reference searchable in the source code.

- For a type of Request, a `TestRequest` function is expected,
- For a function Service.Init, a `TestService_Init` function is expected,
- For a function `Flatten`, a `TestFlatten` function is expected.

Using `t.Run` within tests is expected for more fine grained tests.

This structural check isn't currently implemented. The application
couplings with tests vary greatly, and would better live in a separate
linter for the purpose.

## Typical violations

Here are a few typical violations of package structure:

- many globals, making testing scope hard to reason about
- arbitrarily named tests causing duplication
- multiple type definitions in a single file
- a single definition over multiple files
- shared utilities in the same package (globals)
- integration tests not in dedicated packages (white box tests)

The tooling encourages single responsibility and the testing benefits
that come with such practices. It concentrates on sorting exported
package typed structs and the functions bound to them.
