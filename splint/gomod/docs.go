// Package gomod reads a go.mod and a go.sum into the model.
//
// It sits below both parsers because both need it and neither should reach
// the other: it depends on golang.org/x/mod and on nothing else.
//
// Listing the modules of a tree parses a go.mod only far enough to decide
// whether the module is importable. A release comparison needs the rest: the
// go and toolchain directives, and every require line with its pinned
// version. Find walks up from a package directory, since `splint -i model/`
// has to reach the go.mod a level or more above it, and a tree with no go.mod
// still has packages worth reporting.
//
// A go.sum line is a module, a version and a hash, and a version written with
// a "/go.mod" suffix hashes the go.mod of that version and leaves out its
// source. ReadSum reads the two lines as one version, and the suffix says
// whether the build downloads it: the module graph reads the go.mod of every
// version it considers, and downloads the source of the one it selects. The
// hashes secure the download, and a report reads go.sum for the versions
// alone. A version two modules both record is one version in the Catalogue,
// since they resolve against one module cache.
//
// The Catalogue matches an import path to a module on a path boundary:
// "example.com/x" owns "example.com/x/inner" and not "example.com/xylophone",
// and the longest match wins, since a go.mod may require a module alongside a
// submodule of itself. Base folds the major versions of a module together: a
// module requiring both links both into the binary, and the types of one do
// not fit the interfaces of the other.
package gomod
