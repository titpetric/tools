// Package importpolicy reports an import the tree's deny list refuses.
//
// A module host is a decision the dependency graph takes once and every file
// importing it repeats. The deny list starts with gopkg.in: it exists to
// serve a module the Go versioning rule cannot name, it puts the
// major version on the same path segment as the name, and the packages it
// serves have moved to their own repositories under a path the module graph
// reads without a rule of its own. A tree that imports from it keeps a name
// that no tool can place without the special case, which is the case splint
// carries in model and importfmt and would rather not.
//
// The allow list is read first and carves a path out of the deny list, which
// is how a tree keeps the one dependency it cannot drop while refusing the
// host around it. A denied path with a known replacement reports it, so the
// finding reads as a change to make and not as a name to argue with. A denied
// path without one still reports, because the tree said the host is refused
// and the file did not hear it.
//
// Both lists are stated in .splint.yml under importpolicy: allow carves out of
// the defaults and deny adds to them.
package importpolicy

import (
	"context"
	"fmt"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// Name selects the linter and labels its issues.
const Name = "importpolicy"

// RuleDenied is the one rule this linter reports under.
const RuleDenied = "denied"

// Deny is one import path family the tree refuses and what to import in its
// place.
type Deny struct {
	// Prefix is the import path or path prefix the entry covers. It covers the
	// path it names and everything below it, so gopkg.in/yaml.v3 covers
	// gopkg.in/yaml.v3/decode and not gopkg.in/yaml.v30.
	Prefix string

	// Replacement is the import path to use instead, and is empty when the
	// tree names none: the finding then says the host is refused and names no
	// substitute.
	Replacement string
}

// DefaultDenyList are the families the linter refuses with no tree setting.
// The yaml entries name goccy/go-yaml, which reads the same documents through
// the API splint already uses, so the replacement is the codec the tree moved
// to and not a rewrite of every call.
var DefaultDenyList = []Deny{
	{Prefix: "gopkg.in/yaml.v2", Replacement: "github.com/goccy/go-yaml"},
	{Prefix: "gopkg.in/yaml.v3", Replacement: "github.com/goccy/go-yaml"},
	{Prefix: "gopkg.in/check.v1", Replacement: "github.com/stretchr/testify"},
	{Prefix: "gopkg.in"},
}

// Linter reports the imports a tree refuses.
type Linter struct {
	// AllowList are the path prefixes the deny list does not reach. An entry
	// here beats every entry of the deny list.
	AllowList []string

	// DenyList are the path families the linter reports. The first entry
	// covering a path applies, so a replacement sits above the host entry it
	// carves out of.
	DenyList []Deny
}

// New returns the linter under the default deny list.
func New() *Linter {
	return &Linter{DenyList: append([]Deny(nil), DefaultDenyList...)}
}

// Name is the linter name.
func (l *Linter) Name() string {
	return Name
}

// Lint reports every import of the document the deny list covers.
func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	var results Results

	for _, def := range root.Packages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		for _, file := range def.Imports.Keys() {
			for _, literal := range def.Imports[file] {
				path := importPath(literal)
				deny, denied := l.denied(path)
				if !denied {
					continue
				}

				results.add(Result{
					Rule:        RuleDenied,
					Symbol:      path,
					Position:    model.Position{Package: def.Package.Package, File: filePath(def.Package, file)},
					Replacement: deny.Replacement,
					Message:     message(path, deny.Replacement),
				})
			}
		}
	}

	return results, nil
}

// denied returns the deny entry covering path, the allow list read first.
func (l *Linter) denied(path string) (Deny, bool) {
	for _, prefix := range l.AllowList {
		if covers(prefix, path) {
			return Deny{}, false
		}
	}
	for _, deny := range l.DenyList {
		if covers(deny.Prefix, path) {
			return deny, true
		}
	}
	return Deny{}, false
}

// covers reports whether a prefix names a path or a directory above it.
func covers(prefix, path string) bool {
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		return false
	}
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// message says what to do about an import the deny list covers.
func message(path, replacement string) string {
	if replacement != "" {
		return fmt.Sprintf("%s is on the deny list: import %s instead", path, replacement)
	}
	return fmt.Sprintf("%s is on the deny list, which names no replacement for it: depend on the module where its repository serves it", path)
}

// importPath strips the alias and the quotes an import literal is recorded with.
func importPath(literal string) string {
	if at := strings.LastIndex(literal, " "); at >= 0 {
		literal = literal[at+1:]
	}
	return strings.Trim(literal, `"`)
}

// filePath names a file as a Position does: the package directory and the name.
func filePath(pkg model.Package, file string) string {
	dir := strings.TrimPrefix(strings.TrimPrefix(pkg.Path, "."), "/")
	if dir == "" {
		return file
	}
	return dir + "/" + file
}
