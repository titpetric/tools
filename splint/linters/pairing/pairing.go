// Package pairing reports a file with no test beside it.
//
// A file is excused without one when every exported symbol it declares is
// referenced by a test somewhere in the document: the tests of such a file
// exist, they just sit in another package, which is a layout rather than a
// gap.
//
// It is a port of the gofsck analyzer of the same name, reimplemented against
// the splint model: the check is the same idea and the reading is different,
// because a document is not a syntax tree.
package pairing

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/refindex"
)

// Name is how the linter is selected and how its issues are labelled.
const Name = "pairing"

// RuleUnpaired is the one rule this linter reports under.
const RuleUnpaired = "unpaired"

// Linter reports a file with no test beside it.
type Linter struct{}

// New returns the linter.
func New() *Linter {
	return &Linter{}
}

// Name is the linter name.
func (l *Linter) Name() string {
	return Name
}

// Lint pairs every file with the test named after it.
//
// A package and the package holding its tests arrive as two definitions of the
// same directory, so the files are gathered by directory before anything is
// paired: the foo_test.go recorded against the test half has to meet the
// foo.go recorded against the other one, and neither definition sees both.
func (l *Linter) Lint(ctx context.Context, root *model.DocumentRoot) (model.LintReport, error) {
	var (
		results Results
		groups  = map[string]*group{}
		order   []string
	)

	for _, def := range root.Packages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		key := def.Package.Path
		if key == "" {
			key = def.Package.ImportPath
		}

		found, known := groups[key]
		if !known {
			found = newGroup()
			groups[key] = found
			order = append(order, key)
		}
		found.read(def)
	}

	index := refindex.Build(root)

	// The groups are reported in the order the document named them, so a run
	// over the same tree reports the same issues in the same order.
	for _, key := range order {
		groups[key].report(index, &results)
	}

	return results, nil
}

// group is the files of one directory, however many definitions they arrived
// in, split into the half a test binary compiles and the half it does not.
type group struct {
	pkg    model.Package
	named  bool
	files  map[string]model.File
	tested map[string]bool

	// symbols are the exported declarations of each file, keyed the way the
	// files are, so a standalone file can be asked for elsewhere-tested.
	symbols map[string][]refindex.Key
}

// newGroup returns a group ready to be filled.
func newGroup() *group {
	return &group{
		files:   map[string]model.File{},
		tested:  map[string]bool{},
		symbols: map[string][]refindex.Key{},
	}
}

// read takes the files of one definition into the group.
//
// The definition that is not the test package names the directory the way an
// importer would, so it is the one the group is named after: the test half of
// "x" is recorded as "x_test", which is an import path nothing outside the
// test binary has.
func (g *group) read(def *model.Definition) {
	if !g.named || (g.pkg.TestPackage && !def.Package.TestPackage) {
		g.pkg = def.Package
		g.named = true
	}

	for _, file := range def.Files {
		if file.Generated {
			continue
		}
		if file.Test {
			g.tested[file.Base()] = true
			continue
		}
		g.files[file.Base()] = file
	}

	// The exported declarations of every non-test file, recorded against the
	// same key the file is, so report can ask whether the file's whole
	// surface is tested from somewhere else.
	for _, decl := range def.Order() {
		if decl.IsTestScope() || !decl.IsExported() {
			continue
		}
		if file, known := def.Files.Find(decl.File); known && file.Generated {
			continue
		}
		base := strings.TrimSuffix(decl.File, ".go")
		g.symbols[base] = append(g.symbols[base], refindex.Key{
			ImportPath: def.Package.ImportPath,
			Symbol:     decl.Symbol(),
		})
	}
}

// report counts the group and reports the files nothing tests.
//
// A standalone file whose exported symbols are all referenced by tests is
// counted rather than reported: its tests exist, in whatever package they
// were written in. A file exporting nothing has no surface to excuse it, and
// is reported the way it always was.
func (g *group) report(index *refindex.Index, results *Results) {
	metric := results.count(g.pkg, len(g.files), len(g.tested))

	paired := 0
	var standalone []string
	for base := range g.files {
		if g.tested[base] {
			paired++
			continue
		}
		standalone = append(standalone, base)
	}

	// A map is walked in no order, so the findings are put back into the order
	// a reader would list the directory in.
	sort.Strings(standalone)

	metric.Paired += paired

	// A test naming no file is counted and not reported: it is worth knowing
	// about, and there is no file to hang an issue off.
	metric.StandaloneTests += len(g.tested) - paired

	for _, base := range standalone {
		if g.testedElsewhere(index, base) {
			metric.TestedElsewhere++
			continue
		}
		file := g.files[base]
		results.add(metric, Result{
			Rule:     RuleUnpaired,
			Position: position(g.pkg, file),
			Message:  fmt.Sprintf("%s has no %s_test.go beside it", file.Name, base),
		})
	}
}

// testedElsewhere reports a file whose exported symbols are all referenced by
// a test, wherever the test sits.
func (g *group) testedElsewhere(index *refindex.Index, base string) bool {
	keys := g.symbols[base]
	if len(keys) == 0 {
		return false
	}
	for _, key := range keys {
		if !index.TestedDirect(key) {
			return false
		}
	}
	return true
}

// position is where a file is, relative to the root of the parse. The file
// records its base name and the package records the directory, so neither says
// on its own where a reader should look.
func position(pkg model.Package, file model.File) model.Position {
	dir := strings.TrimPrefix(strings.TrimPrefix(pkg.Path, "."), "/")
	name := file.Name
	if dir != "" {
		name = dir + "/" + name
	}
	return model.Position{Package: pkg.Package, File: name}
}
