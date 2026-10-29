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
// packages it calls into, the literals it writes, and every function of the
// module it calls, its own helpers and the code it tests alike, followed down
// to the call that crosses. A guess that is wrong one way is a fake that
// looks like a real thing, or a branch the test never takes; the other way is
// a call through an interface or a func value, which names no function to
// follow.
package integration

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/titpetric/tools/splint/model"
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
	// RuleTestMain is a TestMain that reaches a database, which makes every
	// test of its package an integration test.
	RuleTestMain = "testmain"
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

	// Depth is how many calls into the module a test's chain may run before
	// the call that crosses, and still read as crossing.
	Depth int
}

// DefaultDepth is the longest chain of calls a finding follows. Over task-ui
// it keeps precision and recall both above 98%: a deeper chain is most often
// a branch the test never takes.
const DefaultDepth = 3

// New returns the linter, switched off, under the default tag.
func New() *Linter {
	return &Linter{Tag: DefaultTag, Depth: DefaultDepth}
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

	// The scope reads the files behind the tag too: a TestMain there runs
	// for the integration tests of its package, and the testmain rule asks
	// about it as well.
	tagged := root.WithTagged(tag)
	scope := newScope(tagged, l.storage)
	for _, dir := range directories(root.Packages) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		l.lintDir(&results, scope, dir, tag)
	}
	for _, def := range tagged.Packages {
		if def != nil && def.TestPackage {
			l.lintTestMain(&results, scope, def, tag)
		}
	}

	return results, nil
}

// lintTestMain reports a TestMain that reaches a database: it runs before
// every test of the package, so each of them needs one, and the package is
// an integration package under any file names.
//
// The rule lets a package be whose name says it holds such tests: e2e, or a name
// holding "test", such as skillstest or testutil. The name is the one the
// package declares, with the _test suffix of a test package taken off, so
// store_test reads as store and is not excused.
func (l *Linter) lintTestMain(results *Results, scope *scope, def *model.Definition, tag string) {
	name := strings.TrimSuffix(def.Package.Package, "_test")
	if name == "e2e" || strings.Contains(name, "test") {
		return
	}

	for _, decl := range def.Funcs {
		if decl.Name != "TestMain" || decl.Receiver != "" || !decl.IsTestScope() {
			continue
		}
		reason := scope.databaseReason(decl, l.Depth)
		if reason == "" {
			continue
		}
		results.add(Result{
			Rule:     RuleTestMain,
			Severity: model.SeverityError,
			Symbol:   decl.Name,
			Position: decl.Position(def.Package),
			Reason:   reason,
			Message: fmt.Sprintf("TestMain %s: every test of %s needs a database; move them behind //go:build %s, or name the package e2e or *test*",
				reason, name, tag),
		})
	}
}

// lintDir applies the file rules and boundary to one directory.
func (l *Linter) lintDir(results *Results, scope *scope, dir []*model.Definition, tag string) {
	metric := results.count(dir[0].Package)
	path := dirPath(dir[0].Package)

	for _, def := range dir {
		for _, name := range def.Excluded.Keys() {
			if !model.BehindTag(def.Excluded[name], tag) {
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
		}
	}

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
// the boundary.
func (l *Linter) lintFile(results *Results, metric *Metric, scope *scope, def *model.Definition, file model.File, tag string) {
	fix := fmt.Sprintf("move it to %s with //go:build %s", strings.TrimSuffix(file.Name, "_test.go")+suffix, tag)

	for _, decl := range def.Funcs {
		if decl.File != file.Name || !isTest(decl) {
			continue
		}
		metric.Tests++
		reason := scope.reason(decl, l.Depth)
		if reason == "" {
			continue
		}
		metric.Crossing++
		results.add(Result{
			Rule:     RuleBoundary,
			Severity: model.SeverityWarn,
			Symbol:   decl.Name,
			Position: decl.Position(def.Package),
			Reason:   reason,
			Message:  fmt.Sprintf("%s %s; %s", decl.Name, reason, fix),
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
