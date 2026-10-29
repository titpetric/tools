// Package blacklist reports an import from a module host the tree discourages.
//
// A module host is a decision the dependency graph takes once and every file
// importing it repeats. gopkg.in is the one this linter starts with: it exists
// to serve a module the Go versioning rule cannot name, it puts the major
// version on the same path segment as the name, and the packages it serves
// have moved to their own repositories under a path the module graph reads
// without a rule of its own. A tree that imports from it keeps a name that no
// tool can place without the special case, which is the case splint carries in
// model and importfmt and would rather not.
//
// A blocked path with a known replacement reports it, so the finding reads as
// a change to make and not as a name to argue with. A blocked path without one
// still reports, because the tree said the host is discouraged and the file
// did not hear it.
package blacklist

import (
	"context"
	"fmt"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// Name selects the linter and labels its issues.
const Name = "blacklist"

// RuleBlocked is the one rule this linter reports under.
const RuleBlocked = "blocked"

// Block is one import path family a tree discourages and what to import in
// its place.
type Block struct {
	// Prefix is the import path or path prefix the block covers. It covers
	// the path it names and everything below it, so gopkg.in/yaml.v3 covers
	// gopkg.in/yaml.v3/decode and not gopkg.in/yaml.v30.
	Prefix string

	// Replacement is the import path to use instead, and is empty when the
	// linter knows none: the finding then says the host is discouraged and
	// names no substitute.
	Replacement string
}

// DefaultBlocks are the families the linter blocks unless a tree says
// otherwise. The yaml entries name goccy/go-yaml, which reads the same
// documents through the API splint already uses, so the replacement is the
// codec the tree moved to and not a rewrite of every call.
var DefaultBlocks = []Block{
	{Prefix: "gopkg.in/yaml.v2", Replacement: "github.com/goccy/go-yaml"},
	{Prefix: "gopkg.in/yaml.v3", Replacement: "github.com/goccy/go-yaml"},
	{Prefix: "gopkg.in/check.v1", Replacement: "github.com/stretchr/testify"},
	{Prefix: "gopkg.in"},
}

// Linter reports the imports a tree blocks.
type Linter struct {
	// Blocks are the path families the linter reports. The first block
	// covering a path applies, so a replacement sits above the host entry
	// it carves out of.
	Blocks []Block
}

// New returns the linter under the default blocks.
func New() *Linter {
	return &Linter{Blocks: append([]Block(nil), DefaultBlocks...)}
}

// Name is the linter name.
func (l *Linter) Name() string {
	return Name
}

// Lint reports every import of the document a block covers.
func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	var results Results

	for _, def := range root.Packages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		for _, file := range def.Imports.Keys() {
			for _, literal := range def.Imports[file] {
				path := importPath(literal)
				block, blocked := l.block(path)
				if !blocked {
					continue
				}

				results.add(Result{
					Rule:        RuleBlocked,
					Symbol:      path,
					Position:    model.Position{Package: def.Package.Package, File: filePath(def.Package, file)},
					Replacement: block.Replacement,
					Message:     message(path, block.Replacement),
				})
			}
		}
	}

	return results, nil
}

// block returns the first block covering path, and whether one does.
func (l *Linter) block(path string) (Block, bool) {
	for _, block := range l.Blocks {
		prefix := strings.TrimSuffix(block.Prefix, "/")
		if prefix == "" {
			continue
		}
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return block, true
		}
	}
	return Block{}, false
}

// message says what to do about an import from a blocked path.
func message(path, replacement string) string {
	if replacement != "" {
		return fmt.Sprintf("%s is from a blocked module host: import %s instead", path, replacement)
	}
	return fmt.Sprintf("%s is from a blocked module host, which has no replacement in the tree: depend on the module where its repository serves it", path)
}

// importPath is the path of an import literal, with the alias and the quotes
// taken off. Definition.Imports records a literal as the file writes it.
func importPath(literal string) string {
	if at := strings.LastIndex(literal, " "); at >= 0 {
		literal = literal[at+1:]
	}
	return strings.Trim(literal, `"`)
}

// filePath names a file the way a Position names one, the package directory
// and the filename.
func filePath(pkg model.Package, file string) string {
	dir := strings.TrimPrefix(strings.TrimPrefix(pkg.Path, "."), "/")
	if dir == "" {
		return file
	}
	return dir + "/" + file
}
