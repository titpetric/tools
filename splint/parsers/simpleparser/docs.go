// Package simpleparser reads Go source into the model without building a
// syntax tree.
//
// A file is one the build reads when its name does not end in a platform the
// build is not for, "handler_windows.go" on linux, and its build constraint
// evaluates true. Both are what the toolchain applies before a compiler sees
// the file, so a parser reading the file anyway would describe a package
// nobody builds. go/build/constraint evaluates the constraint without
// building anything: a tag other than the platform reads as unset, the same
// as a plain "go build" does with it.
//
// The parse records the test files a build constraint keeps out under
// Definition.Excluded, and ReadTagged reads them the way the parser reads any
// file: what they declare and reach, the facts a linter counting the tests
// of a file reads. The ast parser fills the same field from a second load
// under the tags the constraints name, which resolves the methods a test
// calls as well.
package simpleparser
