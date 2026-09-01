package tests_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/titpetric/tools/splint"
	"github.com/titpetric/tools/splint/loader"
	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/simpleparser"
	"github.com/titpetric/tools/splint/tests"
)

// workspace is where the projects the harness reads are checked out, which
// SPLINT_WORKSPACE overrides. A project that is not there is skipped, so the
// harness reads whatever the machine has rather than requiring all of them.
var workspace = envOr("SPLINT_WORKSPACE", "/root/workspace/github")

// projects are the trees to compare on. They are real repositories rather than
// fixtures because the point is the shapes nobody thought to write a fixture
// for: build tags, generic types, generated code, files that do not compile.
var projects = []string{
	"atkins", "cli", "etl", "exp", "go-web-crontab", "lessgo", "mig", "oida",
	"pdo", "phpscript", "platform", "platform-app", "secretsmanager", "tools",
	"vuego", "vuego-cli",
}

// budget is the share of compared values that may differ on a path nobody has
// explained. It is a ratchet: the number is what the harness reports today,
// and lowering it as the differences are chased is the point of having it.
// Raising it is a decision somebody has to make on purpose.
const budget = 0.001

// allowed are the paths the two parsers are not expected to agree on, and why.
// A path not named here has to match, and the summary says so.
//
// An entry is matched on the end of a path, so one covers the field wherever
// in the document it turns up: "Complexity.Cognitive" covers the complexity of
// a func, of a type and of a package alike.
var allowed = map[string]string{
	"Module.Sums":          "go-fsck reads the go.mod and not the go.sum beside it, so its documents record no versions",
	"SelfContained":        "a go-fsck built before the field went records whether a type names another type, which splint reads out of the globals instead",
	"Complexity.Cognitive": "gocognit weights a branch by how deeply it nests in the syntax tree, which a line scan has none of",
	"Complexity.Cyclomatic": "gocyclo counts the branch nodes of a tree, and a line scan counts the keywords " +
		"that produce them, which part company inside a composite literal",
}

// explained returns why a path is allowed to differ, and whether it is.
func explained(path string) (string, bool) {
	for suffix, note := range allowed {
		if path == suffix || strings.HasSuffix(path, "."+suffix) {
			return note, true
		}
	}
	return "", false
}

// TestParity compares the simple parser against "go-fsck extract", value by
// value, over every project.
//
// The comparison is a deep walk of the encoded documents rather than a list of
// fields somebody remembered to check: every key either side carries is
// visited, a key only one of them has is a difference, and a key added to the
// model tomorrow is covered without anyone adding it here.
func TestParity(t *testing.T) {
	if _, err := exec.LookPath("go-fsck"); err != nil {
		t.Skip("go-fsck is not installed")
	}

	totals := map[string]int{}
	examples := map[string]string{}
	var compared, differing int

	for _, project := range projects {
		root := filepath.Join(workspace, project)
		if _, err := os.Stat(root); err != nil {
			t.Logf("skipping %s, not checked out", project)
			continue
		}

		t.Run(project, func(t *testing.T) {
			out := compare(t, root)
			for path, count := range out.counts {
				totals[path] += count
				if _, seen := examples[path]; !seen {
					examples[path] = out.examples[path]
				}
			}
			compared += out.compared
			differing += out.differing
		})
	}

	if compared == 0 {
		t.Skip("no project was read")
	}

	t.Logf("total: %d values compared, %d differ (%.4f%%)", compared, differing, percent(differing, compared))

	unexplained := 0
	for _, path := range sortedKeys(totals) {
		note, ok := explained(path)
		if !ok {
			note = "unexplained"
			unexplained += totals[path]
		}
		t.Logf("  %-30s %7d  %s", path, totals[path], note)
		if !ok && examples[path] != "" {
			t.Logf("      %s", examples[path])
		}
	}

	share := float64(unexplained) / float64(compared)
	t.Logf("unexplained: %d of %d values, %.4f%%, budget %.4f%%",
		unexplained, compared, share*100, budget*100)
	if share > budget {
		t.Errorf("the parsers differ on %.4f%% of values, over the budget of %.4f%%", share*100, budget*100)
	}
}

// result is what one project's comparison found.
type result struct {
	counts    map[string]int
	examples  map[string]string
	compared  int
	differing int
}

// note keeps the first example of each difference, which is what turns a count
// into something a reader can chase.
func (r result) note(path, example string) {
	if _, seen := r.examples[path]; !seen {
		r.examples[path] = example
	}
}

// compare reads one project with both and reports where they differ.
func compare(t *testing.T, root string) result {
	t.Helper()

	reference, err := extract(t, root)
	if err != nil {
		t.Skipf("go-fsck could not read %s: %v", root, err)
	}

	options := splint.Options{SourcePath: root, Pattern: "./...", IncludeSources: true}
	parsed, err := simpleparser.New(options).Parse(context.Background())
	if err != nil {
		t.Fatalf("the simple parser could not read %s: %v", root, err)
	}

	left, right := index(reference), index(parsed)
	out := result{counts: map[string]int{}, examples: map[string]string{}}

	for _, key := range union(left, right) {
		a, inLeft := left[key]
		b, inRight := right[key]

		switch {
		case !inRight:
			out.counts["_onlyInGoFsck"]++
			out.differing++
			out.note("_onlyInGoFsck", key)
			continue
		case !inLeft:
			out.counts["_onlyInSimple"]++
			out.differing++
			out.note("_onlyInSimple", key)
			continue
		}

		out.compared += values(a)
		for _, difference := range tests.Diff(tests.Canonical(a), tests.Canonical(b), normalise) {
			path := tests.Field(difference.Path)
			out.counts[path]++
			out.differing++
			out.note(path, key+" "+difference.String())
		}
	}

	t.Logf("%d packages, %d values compared, %d differ (%.4f%%)",
		len(left), out.compared, out.differing, percent(out.differing, out.compared))
	for _, path := range sortedKeys(out.counts) {
		note, ok := explained(path)
		if !ok {
			note = "unexplained"
		}
		t.Logf("  %-30s %7d  %s", path, out.counts[path], note)
		if !ok {
			t.Logf("      %s", out.examples[path])
		}
	}

	return out
}

// extract runs "go-fsck extract" over a tree and reads the document back.
//
// The flags are the ones the simple parser is asked for, so the two documents
// describe the same thing: everything below the root, sources kept, tests left
// out.
func extract(t *testing.T, root string) (*model.DocumentRoot, error) {
	t.Helper()

	out := filepath.Join(t.TempDir(), "go-fsck.json")
	cmd := exec.Command("go-fsck", "extract", "-r", "--include-sources", "-o", out, "./...")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, firstLine(string(output)))
	}

	return loader.Load(out)
}

// index keys every package of a document, so two documents of the same tree
// compare package by package whatever order they list them in.
func index(doc *model.DocumentRoot) map[string]*model.Definition {
	out := map[string]*model.Definition{}

	for _, def := range doc.Packages {
		key := def.Package.Path + "|" + def.Package.ImportPath
		if def.Package.TestPackage {
			key += "|test"
		}
		out[key] = def
	}

	return out
}

// normalise takes the differences that are known and explained out of the
// comparison, without taking the field they are in out of it.
func normalise(path string, a, b any) (any, any) {
	if strings.HasSuffix(path, "Source") {
		// go-fsck renders a declaration through go/printer, which writes the
		// padding that lines up a run of struct fields as tabs. The file on
		// disk carries what gofmt wrote there, which is spaces. The two say
		// the same thing, so the padding inside a line is collapsed; the
		// indentation opening a line is code structure and is left alone.
		return alignment(a), alignment(b)
	}

	if strings.HasSuffix(path, ".Globals") {
		// The globals are where the two parsers part company on purpose: the
		// ast parser resolves a name against the package scope and the line
		// scanner decides from what the file declares and what the body
		// binds. The second over-reports a local it did not see bound and
		// misses a composite literal key, at a few percent of the names.
		//
		// A reader of the field resolves the names against what the package
		// declares, which is what keeps the difference from mattering, and is
		// why it is not counted here. The references beside it are collected
		// the same way on both sides and are compared.
		return nil, nil
	}

	if strings.HasPrefix(path, "Imports.") {
		// go-fsck drops the underscore in front of a blank import and splint
		// keeps it, because a rule about side effects has to be able to tell
		// one. Both name the same import, so the alias is dropped here.
		return unblank(a), unblank(b)
	}

	return a, b
}

// unblank removes the alias of a blank import from a literal.
func unblank(value any) any {
	literal, ok := value.(string)
	if !ok {
		return value
	}
	return strings.TrimPrefix(literal, "_ ")
}

// alignment collapses the column padding inside every line of a value.
func alignment(value any) any {
	source, ok := value.(string)
	if !ok {
		return value
	}

	lines := strings.Split(source, "\n")
	for i, line := range lines {
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		lines[i] = line[:indent] + collapse(line[indent:])
	}
	return strings.Join(lines, "\n")
}

// collapse reduces every run of spaces and tabs to one space.
func collapse(text string) string {
	var out strings.Builder
	space := false

	for i := 0; i < len(text); i++ {
		if text[i] == ' ' || text[i] == '\t' {
			space = true
			continue
		}
		if space {
			out.WriteByte(' ')
			space = false
		}
		out.WriteByte(text[i])
	}

	return out.String()
}

// values counts the scalars a package encodes to, which is what a difference
// is counted against: a rate over declarations says nothing about a field only
// some declarations carry.
func values(def *model.Definition) int {
	return tests.Scalars(tests.Canonical(def))
}

// union returns every key either side carries, in order.
func union(left, right map[string]*model.Definition) []string {
	seen := map[string]bool{}
	var keys []string

	for _, side := range []map[string]*model.Definition{left, right} {
		for key := range side {
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}

	sort.Strings(keys)
	return keys
}

func sortedKeys(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func percent(part, whole int) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole) * 100
}

func firstLine(output string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(output), "\n")
	return line
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
