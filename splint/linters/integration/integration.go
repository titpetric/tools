// Package integration reports a test that crosses a process boundary outside
// the build tag that keeps such tests out of a plain "go test".
//
// A test crosses the boundary when it runs a process, queries a database,
// sqlite on ":memory:" included, or listens on or dials a socket. Everything
// else is a unit test: code in the process, fakes behind interfaces,
// httptest.NewRecorder and a handler's ServeHTTP, files under t.TempDir. The
// house rule puts the first kind in "<name>_integration_test.go" files opening
// on "//go:build integration", so the default run is the unit tests alone and
// the expensive ones run under -tags integration, on demand.
//
// The linter reads the model and guesses from what a test reaches: the
// packages it calls into, the literals it writes and the helpers of its own
// test files it calls. A guess that is wrong one way is a fake that looks
// like a real thing; the other way is a boundary reached through code the
// test does not write, such as a production function that execs, which the
// linter does not follow.
package integration

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/pathmatch"
)

// Name selects the linter and labels its issues.
const Name = "integration"

// The rules the linter reports under.
const (
	// RuleBoundary is a test outside the tag that crosses a process boundary.
	RuleBoundary = "boundary"
	// RuleFilename is a file behind the tag not named _integration_test.go.
	RuleFilename = "filename"
	// RuleTag is a file named _integration_test.go that the tag does not keep
	// out of a plain build.
	RuleTag = "tag"
	// RuleOutside is an integration test in a package the tree does not list
	// as one that may hold them.
	RuleOutside = "outside"
)

// DefaultTag is the build tag integration tests sit behind.
const DefaultTag = "integration"

// suffix is how the file of an integration test ends its name.
const suffix = "_integration_test.go"

// Linter reports tests crossing a process boundary outside the build tag.
type Linter struct {
	// Enabled turns the linter on. A tree opts in through its .splint.yml,
	// or a run names the linter: a tree that never split its tests would
	// otherwise read every test it has as a finding.
	Enabled bool

	// Tag is the build tag the integration tests sit behind.
	Tag string

	// Storage are the import paths of the packages whose constructors open
	// the real storage. An empty list reads any path ending in /storage or
	// /store as one.
	Storage []string

	// Allow are the package directories that may hold integration tests, as
	// path patterns. An empty list turns the outside rule off.
	Allow []string
}

// New returns the linter, switched off, under the default tag.
func New() *Linter {
	return &Linter{Tag: DefaultTag}
}

// Name is the linter name.
func (l *Linter) Name() string {
	return Name
}

// Lint reads every directory of the tree: its test functions, the helpers
// they call, and the test files a build constraint keeps out.
func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	var results Results
	if !l.Enabled {
		return results, nil
	}

	tag := l.Tag
	if tag == "" {
		tag = DefaultTag
	}

	for _, dir := range directories(root.Packages) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		l.lintDir(&results, dir, tag)
	}

	return results, nil
}

// lintDir applies the four rules to one directory.
func (l *Linter) lintDir(results *Results, dir []*model.Definition, tag string) {
	metric := results.count(dir[0].Package)
	path := dirPath(dir[0].Package)

	for _, def := range dir {
		for _, name := range def.Excluded.Keys() {
			if !behindTag(def.Excluded[name], tag) {
				continue
			}
			metric.Integration++
			position := model.Position{Package: def.Package.Package, File: join(path, name)}
			if !strings.HasSuffix(name, suffix) {
				results.add(Result{
					Rule:     RuleFilename,
					Severity: model.SeverityError,
					Symbol:   name,
					Position: position,
					Message: fmt.Sprintf("%s sits behind //go:build %s; name it %s",
						name, tag, strings.TrimSuffix(name, "_test.go")+suffix),
				})
			}
			if !l.allowed(path) {
				results.add(Result{
					Rule:     RuleOutside,
					Severity: model.SeverityInfo,
					Symbol:   name,
					Position: position,
					Message:  fmt.Sprintf("integration test outside the packages allowed to hold them (%s)", strings.Join(l.Allow, ", ")),
				})
			}
		}
	}

	scope := newScope(dir, l.storage)
	for _, def := range dir {
		for _, file := range def.Files {
			if !file.Test || file.Generated {
				continue
			}
			position := model.Position{Package: def.Package.Package, File: join(path, file.Name)}
			if strings.HasSuffix(file.Name, suffix) {
				results.add(Result{
					Rule:     RuleTag,
					Severity: model.SeverityError,
					Symbol:   file.Name,
					Position: position,
					Message:  fmt.Sprintf("%s does not open on //go:build %s, so a plain go test runs it", file.Name, tag),
				})
				continue
			}
			l.lintFile(results, metric, scope, def, file, tag)
		}
	}
}

// lintFile reports every test function of one untagged file that crosses
// the boundary, and the file itself when only its imports do.
func (l *Linter) lintFile(results *Results, metric *Metric, scope *scope, def *model.Definition, file model.File, tag string) {
	fix := fmt.Sprintf("move it to %s with //go:build %s", strings.TrimSuffix(file.Name, "_test.go")+suffix, tag)

	found := false
	for _, decl := range def.Funcs {
		if decl.File != file.Name || !isTest(decl) {
			continue
		}
		metric.Tests++
		reason := scope.reason(decl)
		if reason == "" {
			continue
		}
		metric.Crossing++
		found = true
		results.add(Result{
			Rule:     RuleBoundary,
			Severity: model.SeverityWarn,
			Symbol:   decl.Name,
			Position: decl.Position(def.Package),
			Reason:   reason,
			Message:  fmt.Sprintf("%s %s; %s", decl.Name, reason, fix),
		})
	}
	if found {
		return
	}

	if reason := scope.fileReason(file.Name); reason != "" {
		results.add(Result{
			Rule:     RuleBoundary,
			Severity: model.SeverityWarn,
			Symbol:   file.Name,
			Position: model.Position{Package: def.Package.Package, File: join(dirPath(def.Package), file.Name)},
			Reason:   reason,
			Message:  fmt.Sprintf("%s %s; %s", file.Name, reason, fix),
		})
	}
}

// storage reports an import path whose constructors open the real storage.
func (l *Linter) storage(path string) bool {
	if len(l.Storage) == 0 {
		return strings.HasSuffix(path, "/storage") || strings.HasSuffix(path, "/store")
	}
	for _, listed := range l.Storage {
		if path == listed {
			return true
		}
	}
	return false
}

// allowed reports a directory that may hold integration tests. With no list,
// every directory may.
func (l *Linter) allowed(path string) bool {
	if len(l.Allow) == 0 {
		return true
	}
	for _, pattern := range l.Allow {
		if ok, _ := pathmatch.Match(pattern, path); ok {
			return true
		}
	}
	return false
}

// directories groups the definitions by the directory holding them: a
// package and its two test scopes share one, and a helper of one test file
// is reachable from every other.
func directories(defs model.DefinitionList) [][]*model.Definition {
	byPath := map[string][]*model.Definition{}
	for _, def := range defs {
		if def == nil {
			continue
		}
		byPath[def.Package.Path] = append(byPath[def.Package.Path], def)
	}

	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	out := make([][]*model.Definition, 0, len(paths))
	for _, path := range paths {
		out = append(out, byPath[path])
	}
	return out
}

// behindTag reports a constraint that holds only with the tag set: the tag
// appears in it and not negated.
func behindTag(exprs []string, tag string) bool {
	for _, expr := range exprs {
		words := strings.FieldsFunc(expr, func(r rune) bool {
			return r == ' ' || r == '(' || r == ')' || r == '&' || r == '|'
		})
		for _, word := range words {
			if word == tag {
				return true
			}
		}
	}
	return false
}

// isTest reports a function the test binary runs: a test, a benchmark, a fuzz
// target, an example, or TestMain.
func isTest(decl *model.Declaration) bool {
	if decl.Receiver != "" || !decl.IsTestScope() {
		return false
	}
	for _, prefix := range []string{"Test", "Benchmark", "Fuzz", "Example"} {
		if strings.HasPrefix(decl.Name, prefix) {
			return true
		}
	}
	return false
}

// dirPath is the package directory as a report names it, with no leading dot
// or slash.
func dirPath(pkg model.Package) string {
	return strings.TrimPrefix(strings.TrimPrefix(pkg.Path, "."), "/")
}

// join names a file of a directory the way a Position names one.
func join(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}
