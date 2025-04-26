# gofsck

A go filesystem check. It checks that exported symbols are appropriately
grouped into matching filenames, or fall into some default groups
inside a given package based on the symbol type.

```
go install github.com/titpetric/tools/gofsck@latest
```

Usage:

```
./gofsck ./...
./gofsck .
```

It's possible to include gofsck to golangci-lint, see the
[.golangci.yml](./.golangci.yml) file for the configuration.

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

## File naming

- A symbol of `ServiceDisovery` is expected in `service_discovery.go`.
- A symbol of `ServiceDiscovery.Get` is expected in `service_discovery_get.go` or `service_discovery.go`.
- The tests for `ServiceDiscovery.Get` should match the name in an adjacent `_test.go` file.

Depending on the declaration kind, additional locations are envisioned:

- constants are global, put them in `consts.go`,
- vars are global, put them in `vars.go`,
- data model goes into `types.go` or own package

## Test naming

The structure is checked with unit tests in mind. Black box unit tests are
a good way to have the full symbol reference searchable in the source code.

- For a type of Request, a `TestRequest` function is expected,
- For a function Service.Init, a `TestService_Init` function is expected,
- For a function `Flatten`, a `TestFlatten` function is expected.

Using `t.Run` within tests is expected for more fine grained tests.

## Typical violations

Here are a few typical violations of package structure:

- many globals, making testing scope hard to reason about
- arbitrarily named tests causing duplication
- multiple type definitions in a single file
- a single definition over multiple files
- shared utilities in the same package
- integration tests not in dedicated packages

The tooling encourages single responsibility and the testing benefits
that come with such practices.
