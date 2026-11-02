package test

import (
	"fmt"
	"go/ast"
	"io"
	"sort"
	"strings"
)

// testID names one test or benchmark.
type testID struct {
	Dir  string
	Name string
}

// selection is what a run will start.
type selection struct {
	Tests   map[testID]bool
	Benches map[testID]bool

	// Changed is every changed symbol, and TotalTests is how many test
	// functions the tree holds. The scan compares the first with the tests
	// that ran from the second.
	Changed      []changedFunc
	TotalTests   int
	TotalBenches int
}

// selectChanged maps every changed function to the tests that exercise it.
//
// A test that changed runs itself. A benchmark that changed runs itself under
// -benchtime 10s. A changed function runs the tests of its own directory that
// name it, Test<Func> and for a method on T every Test<T>*, and the tests that
// reach it through a helper of that package. A test in another package runs
// when its file imports the changed package and its body writes the name.
func selectChanged(inv *inventory, changed []changedFunc, verbose bool, said io.Writer) *selection {
	sel := &selection{Tests: map[testID]bool{}, Benches: map[testID]bool{}, Changed: changed}

	for _, dir := range inv.Dirs {
		for _, tf := range dir.Tests {
			switch tf.Kind {
			case kindBenchmark:
				sel.TotalBenches++
			default:
				sel.TotalTests++
			}
		}
	}

	for i := range changed {
		one := changed[i]
		id := testID{Dir: one.Dir, Name: one.Name}
		switch one.Kind {
		case kindTest, kindFuzz, kindExample:
			sel.Tests[id] = true
			continue
		case kindBenchmark:
			sel.Benches[id] = true
			continue
		}

		matched := sameDir(inv, one, sel)

		// An internal method is reached through the functions on its type.
		// Running their tests covers the method without naming it.
		if !ast.IsExported(one.Name) && one.Receiver != "" {
			if via := invokers(inv, one, sel); len(via) > 0 {
				matched = true
				changed[i].Via = via
			}
		}

		if otherDirs(inv, one, sel) {
			matched = true
		}

		// A name and a call both miss a function whose tests are named for
		// the behaviour they check. The tests of its package are then the
		// tests of the change, and running them is the narrow answer left.
		if !matched {
			selectPackage(inv, one, sel)
			if verbose {
				fmt.Fprintf(said, "splint test: no test names %s, running the tests of %s\n", one.Key, one.Dir)
			}
		}
	}

	return sel
}

// invokers selects the tests that exercise a function on the changed method's
// receiver, and reports whether one did.
func invokers(inv *inventory, one changedFunc, sel *selection) []string {
	index := inv.Dirs[one.Dir]
	if index == nil {
		return nil
	}

	seen := map[string]bool{}
	for name, fns := range index.Funcs {
		if name == one.Name {
			continue
		}
		for _, fn := range fns {
			if fn.Receiver != one.Receiver || fn.Kind != kindFunc {
				continue
			}
			if !fn.Calls[one.Name] && !fn.Refs[one.Name] {
				continue
			}
			if sameDir(inv, changedOf(one.Dir, one.ImportPath, fn), sel) {
				seen[fn.Key] = true
			}
		}
	}

	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// changedOf is a changedFunc for a declaration, used to follow an internal
// method to the functions that call it.
func changedOf(dir, importPath string, fn *funcDecl) changedFunc {
	return changedFunc{
		Dir:        dir,
		Key:        fn.Key,
		Name:       fn.Name,
		Receiver:   fn.Receiver,
		Kind:       fn.Kind,
		ImportPath: importPath,
		StartLine:  fn.StartLine,
		EndLine:    fn.EndLine,
	}
}

// sameDir selects the tests of the changed function's own directory and
// reports whether one named or reached it.
func sameDir(inv *inventory, one changedFunc, sel *selection) bool {
	index := inv.Dirs[one.Dir]
	if index == nil {
		return false
	}

	matched := false
	for _, tf := range index.Tests {
		if tf.Kind == kindBenchmark {
			continue
		}
		if named(one, tf) || reaches(index, tf, one) {
			sel.Tests[testID{Dir: tf.Dir, Name: tf.Name}] = true
			matched = true
		}
	}
	return matched
}

// selectPackage selects every test of the changed function's directory.
func selectPackage(inv *inventory, one changedFunc, sel *selection) {
	index := inv.Dirs[one.Dir]
	if index == nil {
		return
	}
	for _, tf := range index.Tests {
		if tf.Kind == kindBenchmark {
			continue
		}
		sel.Tests[testID{Dir: tf.Dir, Name: tf.Name}] = true
	}
}

// otherDirs selects the tests whose file imports the changed package and
// writes the name, and reports whether one did.
func otherDirs(inv *inventory, one changedFunc, sel *selection) bool {
	if one.ImportPath == "" {
		return false
	}

	matched := false
	for dir, index := range inv.Dirs {
		if dir == one.Dir {
			continue
		}
		for _, tf := range index.Tests {
			if tf.Kind == kindBenchmark {
				continue
			}
			if !tf.Imports[one.ImportPath] {
				continue
			}
			if tf.Refs[one.Name] || tf.Qualified[one.ImportPath+"."+one.Name] {
				sel.Tests[testID{Dir: tf.Dir, Name: tf.Name}] = true
				matched = true
			}
		}
	}
	return matched
}

// named reports the convention that pairs a test with what it tests:
// Test<Func>, and for a method on type T every Test<T>*.
func named(one changedFunc, tf *testFunc) bool {
	if tf.Kind != kindTest {
		return false
	}
	if one.Receiver != "" {
		return strings.HasPrefix(tf.Name, "Test"+one.Receiver)
	}
	return strings.HasPrefix(tf.Name, "Test"+upperFirst(one.Name))
}

// reaches reports whether a test reaches the changed function through the
// helpers of its own package.
//
// A test reads as a description of what it covers, not as the name of the
// function under it: TestRunReportsTheFixture calls runArgs, which calls run.
// The walk follows a test helper's calls and stops at the product code: a
// function the helper calls is a function the test exercises, and what that
// function calls in turn is its own business.
func reaches(index *dirIndex, tf *testFunc, one changedFunc) bool {
	if tf.Refs[one.Name] {
		return true
	}

	seen := map[string]bool{}
	queue := keys(tf.Calls)
	for len(queue) > 0 {
		var next []string
		for _, name := range queue {
			if seen[name] {
				continue
			}
			seen[name] = true
			if name == one.Name {
				return true
			}
			for _, fn := range index.Funcs[name] {
				if fn.Key == one.Key || fn.Refs[one.Name] {
					return true
				}
				if !testHelper(fn) {
					continue
				}
				for call := range fn.Calls {
					if !seen[call] {
						next = append(next, call)
					}
				}
			}
		}
		queue = next
	}
	return false
}

// testHelper reports a function a test file declares to help its tests.
func testHelper(fn *funcDecl) bool {
	return fn.Kind == kindFunc && strings.HasSuffix(fn.File, "_test.go")
}

// keys lists a call set in a stable order.
func keys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// writeList writes what a run would start, one name per line.
func (s *selection) writeList(w io.Writer) error {
	for _, id := range s.ids() {
		if _, err := fmt.Fprintln(w, id.Name); err != nil {
			return err
		}
	}
	return nil
}

// ids is every selected test and benchmark in a stable order.
func (s *selection) ids() []testID {
	out := make([]testID, 0, len(s.Tests)+len(s.Benches))
	for id := range s.Tests {
		out = append(out, id)
	}
	for id := range s.Benches {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Dir < out[j].Dir
	})
	return out
}
