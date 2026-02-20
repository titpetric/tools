# tools - A collection of Go development tools and libraries

### [generic](generic/)

A Go library providing type-safe generic utilities:

- **List[T]** - Generic list type with `Filter`, `Find`, `Get`, `Value`, and `ListMap` operations
- **Pointer[T]** - Helper to get a pointer to any value
- **TemplateRenderer[T]** - Type-safe HTML template rendering with embedded filesystem support and local file overrides

```
go get github.com/titpetric/tools/generic
```

### [gofsck](gofsck/)

A Go filesystem check tool with modular analyzers for package structure validation. Provides three analyzers:

1. **Pairing** - Validates that source files have corresponding test files
2. **Coverage** - Analyzes symbol-test coverage using naming conventions
3. **Grouping** - Ensures exported symbols are in appropriately named files (also available as a golangci-lint plugin)

```
go install github.com/titpetric/tools/gofsck@latest
```

### [puzzle](puzzle/)

A creative tool that visualizes a Go repository's package structure as a crossword puzzle rendered in the terminal. Supports default and matrix rendering styles.

```
go install github.com/titpetric/tools/puzzle@main
```

### [semver](semver/)

A CLI tool that reads `git ls-remote --tags` output from stdin, parses semver tags, and outputs the latest patch version for each minor release across the last two major versions as JSON.

```
go install github.com/titpetric/tools/semver@latest
```

## Development

Each module is an independent Go module. A root [Taskfile.yml](Taskfile.yml) is provided with:

- `task update` - Update Go version and dependencies across all modules
- `task list` - List all sub-module directories

## License

[MIT](LICENSE) - Copyright (c) 2025 Tit Petric
