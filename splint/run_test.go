package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/model/loader"
)

// fixture is the module of code that fails checks on purpose, which is what
// gives these tests something to find.
//
// It is resolved once, and absolute, so a test that moves the process to read
// a tree of its own still points at the same fixture.
var fixture = absolute("testdata")

// absolute resolves a path at startup, before anything has moved.
func absolute(path string) string {
	out, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return out
}

// runArgs runs the command as the shell would and returns what it wrote.
func runArgs(t *testing.T, args ...string) (string, int) {
	t.Helper()

	var out bytes.Buffer
	code, err := run(context.Background(), args, &out, &out)
	if err != nil {
		t.Fatalf("run(%v) error = %v", args, err)
	}
	return out.String(), code
}

// TestRunReportsTheFixture covers the whole of a plain run: the tree is
// parsed, every linter reads it, and what they found is written out.
func TestRunReportsTheFixture(t *testing.T) {
	got, code := runArgs(t, "-i", fixture, "./...")

	if code != exitFound {
		t.Errorf("run() exited %d, want %d: a fixture that fails checks has to fail them", code, exitFound)
	}
	// Redirected output is for a program: one workflow command per finding,
	// which is what GitHub Actions turns into an annotation on a review.
	if !strings.Contains(got, "::warning file=undocumented.go,line=9,title=godoc/missing::Undocumented - ") {
		t.Fatalf("run() wrote no findings:\n%s", got)
	}
	if strings.Contains(got, "\033") {
		t.Error("run() wrote escape codes to something that is not a terminal")
	}
}

// TestRunJSON covers the flag that skips the rendering: the same findings,
// for a reader that is a program.
func TestRunJSON(t *testing.T) {
	got, code := runArgs(t, "--json", "-i", fixture, "./...")

	if code != exitFound {
		t.Errorf("run() exited %d, want %d", code, exitFound)
	}

	var found struct {
		Issues  []model.Issue
		Linters []string
	}
	if err := json.Unmarshal([]byte(got), &found); err != nil {
		t.Fatalf("-json wrote something that is not JSON: %v\n%s", err, got)
	}
	if len(found.Issues) == 0 || len(found.Linters) == 0 {
		t.Fatalf("-json wrote %d issues from %d linters", len(found.Issues), len(found.Linters))
	}
	for _, issue := range found.Issues {
		if issue.Linter == "" || issue.Message == "" {
			t.Errorf("an issue is missing what it says: %#v", issue)
		}
	}
}

// TestRunFlagsAfterThePattern covers the command line written the way a person
// writes it: the flag package stops at the first operand, so a flag after the
// pattern used to be read as a second pattern and the run was silently not the
// one that was asked for.
func TestRunFlagsAfterThePattern(t *testing.T) {
	before, code := runArgs(t, "-i", fixture, "--linters", "godoc", "./...")
	if code != exitFound {
		t.Fatalf("run() exited %d with the flags first", code)
	}

	after, code := runArgs(t, "./...", "-i", fixture, "--linters", "godoc")
	if code != exitFound {
		t.Errorf("run() exited %d with the flags after the pattern", code)
	}
	if after != before {
		t.Errorf("the flags after the pattern read a different run:\n%s\n---\n%s", after, before)
	}
}

// TestRunYAML covers the second encoding: the same answer, written the way a
// document is written.
func TestRunYAML(t *testing.T) {
	got, code := runArgs(t, "--yaml", "-i", fixture, "./...")

	if code != exitFound {
		t.Errorf("run() exited %d, want %d", code, exitFound)
	}

	var found struct {
		Issues  []model.Issue `yaml:"Issues"`
		Linters []string      `yaml:"Linters"`
	}
	if err := yaml.Unmarshal([]byte(got), &found); err != nil {
		t.Fatalf("-yaml wrote something that is not YAML: %v\n%s", err, got)
	}
	if len(found.Issues) == 0 {
		t.Fatalf("-yaml wrote no issues:\n%s", got)
	}

	// A severity is a level, and it reads as the word in both encodings.
	if !strings.Contains(got, "Severity: WARN") {
		t.Errorf("-yaml wrote a severity as something other than the word:\n%s", got)
	}
}

// TestRunOneEncoding covers asking for both: they are two encodings of one
// answer, and a run cannot write both to one stream.
func TestRunOneEncoding(t *testing.T) {
	var out bytes.Buffer
	_, err := run(context.Background(), []string{"--json", "--yaml", "-i", fixture, "./..."}, &out, &out)
	if err == nil {
		t.Fatal("run() accepted both encodings")
	}
	if !strings.Contains(err.Error(), "--json") || !strings.Contains(err.Error(), "--yaml") {
		t.Errorf("run() error = %v", err)
	}
}

// TestRunStatsJSON covers the measurements as data: the numbers behind the
// tables rather than the tables.
func TestRunStatsJSON(t *testing.T) {
	got, code := runArgs(t, "--json", "--stats", "-i", fixture, "./...")

	if code != exitClean {
		t.Errorf("run() exited %d, want %d", code, exitClean)
	}

	var measured []struct {
		Linter     string
		Metrics    model.LintMetrics
		Statistics []model.Statistics
	}
	if err := json.Unmarshal([]byte(got), &measured); err != nil {
		t.Fatalf("-json -stats wrote something that is not JSON: %v\n%s", err, got)
	}
	if len(measured) == 0 {
		t.Fatal("-json -stats wrote nothing")
	}

	for _, one := range measured {
		if one.Linter == "godoc" {
			if one.Metrics.Empty() || len(one.Statistics) == 0 {
				t.Errorf("godoc measured nothing: %#v", one)
			}
			return
		}
	}
	t.Error("-json -stats did not write godoc")
}

// TestRunStats covers the flag that asks what the linters measured rather than
// what they found.
func TestRunStats(t *testing.T) {
	got, code := runArgs(t, "--stats", "-i", fixture, "./...")

	// Statistics are not findings, so a run asking for them is not failing.
	if code != exitClean {
		t.Errorf("run() exited %d, want %d", code, exitClean)
	}
	if strings.Contains(got, "| Position") {
		t.Errorf("-stats wrote the issue table:\n%s", got)
	}
	if !strings.Contains(got, "|--") {
		t.Fatalf("-stats wrote no table:\n%s", got)
	}
	// One blank line between tables and no more.
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("-stats left more than one blank line between tables:\n%q", got)
	}
}

func TestRunParsers(t *testing.T) {
	ast, _ := runArgs(t, "-i", fixture, "--parser=astparser", "./...")
	simple, _ := runArgs(t, "-i", fixture, "--parser=simpleparser", "./...")

	// The two parsers produce the same document, so the linters reading it
	// have to reach the same conclusions.
	if ast != simple {
		t.Errorf("the parsers disagree about the fixture:\n--- ast ---\n%s\n--- simple ---\n%s", ast, simple)
	}
}

func TestRunUnknownParser(t *testing.T) {
	var out bytes.Buffer
	if _, err := run(context.Background(), []string{"--parser=nope", "-i", fixture, "./..."}, &out, &out); err == nil {
		t.Fatal("run() accepted a parser that does not exist")
	} else if !strings.Contains(err.Error(), "astparser") {
		t.Errorf("run() error = %v, want the parsers it does have", err)
	}
}

func TestRunUnknownLinter(t *testing.T) {
	var out bytes.Buffer
	_, err := run(context.Background(), []string{"--linters", "nope", "-i", fixture, "./..."}, &out, &out)
	if err == nil {
		t.Fatal("run() accepted a linter that does not exist")
	}
	// A typo in a flag is a linter silently not running, so it is reported.
	if !strings.Contains(err.Error(), "nope") || !strings.Contains(err.Error(), "godoc") {
		t.Errorf("run() error = %v", err)
	}
}

// TestRunOutputAndInput covers the two halves of reading a document back: a
// run writes one, and another run lints it without parsing anything.
// The written document carries the test packages only when --include-tests
// asked for them, and the linters read them either way, so the two runs report
// the same findings only when the document holds what the parse read.
func TestRunOutputAndInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.json")

	parsed, _ := runArgs(t, "-i", fixture, "--include-tests", "--output", path, "./...")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("--output wrote nothing: %v", err)
	}

	loaded, _ := runArgs(t, "--input", path)
	if parsed != loaded {
		t.Errorf("a document read back reports differently:\n--- parsed ---\n%s\n--- loaded ---\n%s", parsed, loaded)
	}
}

func TestRunSchema(t *testing.T) {
	got, code := runArgs(t, "--schema", "-i", fixture, "./...")

	if code != exitClean {
		t.Errorf("run() exited %d", code)
	}

	var schema struct {
		Schema      string         `json:"$schema"`
		Definitions map[string]any `json:"definitions"`
	}
	if err := json.Unmarshal([]byte(got), &schema); err != nil {
		t.Fatalf("-schema wrote something that is not JSON: %v\n%s", err, got)
	}
	if !strings.Contains(schema.Schema, "json-schema.org") {
		t.Errorf("$schema = %q", schema.Schema)
	}
	// The fixture declares a User and a Thing, so the schema describes them.
	for _, want := range []string{"User", "Thing"} {
		if _, ok := schema.Definitions[want]; !ok {
			t.Errorf("definitions have no %q: %v", want, keys(schema.Definitions))
		}
	}
}

func TestRunHelp(t *testing.T) {
	got, code := runArgs(t, "--help")

	if code != exitClean {
		t.Errorf("run() exited %d", code)
	}
	for _, want := range []string{"-parser NAME", "--stats", "--schema", "astparser", "simpleparser"} {
		if !strings.Contains(got, want) {
			t.Errorf("--help does not mention %q", want)
		}
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	return out
}

// tinyProfile covers the body of Do and nothing else. The line and column
// range is the block the toolchain records for the body of the function
// declared on line 5 of tiny.go.
const tinyProfile = `mode: set
example.com/tiny/tiny.go:5.16,7.2 1 1
`

// TestAppendCoverage covers the overlay end to end: the profile is read during
// the parse, and the document carries what it said.
func TestAppendCoverage(t *testing.T) {
	root := t.TempDir()
	tree := writeTree(t, root, "tree")

	if err := os.WriteFile(filepath.Join(root, "pkg.cov"), []byte(tinyProfile), 0o644); err != nil {
		t.Fatal(err)
	}

	runIn(t, root, "-i", "tree", "--save", "--append-coverage=pkg.cov", ".")

	doc, err := loader.Load(filepath.Join(tree, saveFile))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Packages) != 1 {
		t.Fatalf("the document holds %d packages, want 1", len(doc.Packages))
	}

	def := doc.Packages[0]
	if def.Package.Complexity == nil || def.Package.Complexity.Coverage != 100 {
		t.Errorf("package coverage = %v, want 100", def.Package.Complexity)
	}

	fn := def.Funcs.Find(func(d *model.Declaration) bool { return d.Name == "Do" })
	if fn == nil {
		t.Fatal("the document holds no Do")
	}
	if fn.Complexity == nil || fn.Complexity.Coverage != 100 {
		t.Errorf("Do coverage = %v, want 100", fn.Complexity)
	}
}

// TestAppendCoverageWithInput covers the combination that is refused: the
// overlay reads the line ranges a parse recorded, and -input is the flag that
// skips the parse.
func TestAppendCoverageWithInput(t *testing.T) {
	var out bytes.Buffer
	_, err := run(context.Background(), []string{"--append-coverage=pkg.cov", "--input", "doc.json"}, &out, &out)
	if err == nil {
		t.Fatal("run() accepted --append-coverage with -input")
	}
	if !strings.Contains(err.Error(), "append-coverage") || !strings.Contains(err.Error(), "input") {
		t.Errorf("run() error = %v, want it to name both flags", err)
	}
}

// TestAppendCoverageMissingProfile covers the profile that is not there, which
// is a failed run rather than a document with no coverage in it.
func TestAppendCoverageMissingProfile(t *testing.T) {
	var out bytes.Buffer
	_, err := run(context.Background(), []string{"-i", fixture, "--append-coverage=nowhere.cov", "./..."}, &out, &out)
	if err == nil {
		t.Fatal("run() accepted a profile that does not exist")
	}
	if !strings.Contains(err.Error(), "nowhere.cov") {
		t.Errorf("run() error = %v, want it to name the file", err)
	}
}

// tinySourceV2 removes Do and adds Redo, so a diff of the two trees carries
// one removed and one added symbol and reads as breaking.
const tinySourceV2 = `// Package tiny is one package with one function in it.
package tiny

// Redo returns two.
func Redo() int {
	return 2
}
`

// TestDiffCommand covers the verb end to end: two trees extracted with the
// linters off, diffed as JSON, and the breaking verdict read back.
func TestDiffCommand(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "old")
	tree := writeTree(t, root, "new")
	if err := os.WriteFile(filepath.Join(tree, "tiny.go"), []byte(tinySourceV2), 0o644); err != nil {
		t.Fatal(err)
	}

	runIn(t, root, "--linters", "none", "-i", "old", "--output", "old.json", ".")
	runIn(t, root, "--linters", "none", "-i", "new", "--output", "new.json", ".")

	out := runIn(t, root, "diff", "--old", "old.json", "--new", "new.json", "--json")

	var result struct {
		Removed  []struct{ Key string } `json:"removed"`
		Added    []struct{ Key string } `json:"added"`
		Breaking bool                   `json:"breaking"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("the diff is not JSON: %v:\n%s", err, out)
	}

	if len(result.Removed) != 1 || result.Removed[0].Key != "example.com/tiny.Do" {
		t.Errorf("removed = %+v, want example.com/tiny.Do", result.Removed)
	}
	if len(result.Added) != 1 || result.Added[0].Key != "example.com/tiny.Redo" {
		t.Errorf("added = %+v, want example.com/tiny.Redo", result.Added)
	}
	if !result.Breaking {
		t.Error("a removed exported symbol is a breaking change")
	}

	rendered := runIn(t, root, "diff", "--old", "old.json", "--new", "new.json")
	for _, want := range []string{"- example.com/tiny.Do", "+ example.com/tiny.Redo", "breaking"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("the rendering is missing %q:\n%s", want, rendered)
		}
	}
}

// TestDiffRequiresBothFiles covers the refusal: a diff with one side is not a
// comparison.
func TestDiffRequiresBothFiles(t *testing.T) {
	var out bytes.Buffer
	if _, err := run(context.Background(), []string{"diff", "--old", "old.json"}, &out, &out); err == nil {
		t.Error("diff without --new did not refuse")
	}
}

// TestDocsCommand covers the reference end to end: the tree is parsed with
// sources whether or not they were asked for, and the markdown holds the
// declaration.
func TestDocsCommand(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "tree")

	out := runIn(t, root, "docs", "-i", "tree", ".")

	for _, want := range []string{"# Package tiny", "example.com/tiny", "Do returns one.", "func Do () int"} {
		if !strings.Contains(out, want) {
			t.Errorf("the reference is missing %q:\n%s", want, out)
		}
	}
}

// TestDocsSplit covers split mode: one file per package under --out, and a
// README listing them.
func TestDocsSplit(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "tree")

	runIn(t, root, "docs", "-i", "tree", "--split", "--out", "api", "--strip-prefix", "example.com", ".")

	for _, name := range []string{"README.md", "tiny.md"} {
		if _, err := os.Stat(filepath.Join(root, "api", name)); err != nil {
			t.Errorf("--split wrote no %s: %v", name, err)
		}
	}
}

// TestCoverageCommand covers the report end to end: the profile is folded
// into the parse and the report names the function with its coverage.
func TestCoverageCommand(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "tree")

	if err := os.WriteFile(filepath.Join(root, "pkg.cov"), []byte(tinyProfile), 0o644); err != nil {
		t.Fatal(err)
	}

	out := runIn(t, root, "coverage", "-i", "tree", "--append-coverage=pkg.cov", ".")

	for _, want := range []string{"| Do", "100.00%"} {
		if !strings.Contains(out, want) {
			t.Errorf("the report is missing %q:\n%s", want, out)
		}
	}
}

// TestCoverageTemplate covers --template: the page is the template with the
// two tables in it rather than the function table alone.
func TestCoverageTemplate(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, "tree")

	for file, content := range map[string]string{
		"pkg.cov":     tinyProfile,
		"page.md.tpl": "# Coverage\n\n{{.Packages}}\n\n{{.Functions}}\n",
	} {
		if err := os.WriteFile(filepath.Join(root, file), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	out := runIn(t, root, "coverage", "-i", "tree", "--append-coverage=pkg.cov", "--template=page.md.tpl", ".")

	for _, want := range []string{"# Coverage", "| Do", "| Lines", "100.00%"} {
		if !strings.Contains(out, want) {
			t.Errorf("the page is missing %q:\n%s", want, out)
		}
	}
}

// TestWrittenDocumentHasNoTestPackages covers --include-tests. The parse reads
// the test files whether or not it was asked to, because the pairing and
// coverage linters have nothing to check without them; the document a run
// writes carries them only when the flag asked for it.
func TestWrittenDocumentHasNoTestPackages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.json")
	runArgs(t, "-i", fixture, "--output", path, "./...")

	doc, err := loader.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, def := range doc.Packages {
		if def.TestPackage {
			t.Errorf("the written document holds the test package %s", def.ImportPath)
		}
		for _, decl := range def.DeclarationList() {
			if decl.IsTestScope() {
				t.Errorf("the written document holds %s, declared in %s", decl.Symbol(), decl.File)
			}
		}
		for _, file := range def.Files {
			if file.Test {
				t.Errorf("the written document holds the test file %s", file.Name)
			}
		}
	}

	// The linters still read them: the fixture has a file with no test beside
	// it, which is a finding only when the tests were read.
	found, _ := runArgs(t, "-i", fixture, "--linters", "pairing", "./...")
	if !strings.Contains(found, "pairing/unpaired") {
		t.Error("the pairing linter found nothing, so the parse did not read the tests")
	}
}

// TestWrittenDocumentKeepsTestPackages covers the flag being given.
func TestWrittenDocumentKeepsTestPackages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.json")
	runArgs(t, "-i", fixture, "--include-tests", "--output", path, "./...")

	doc, err := loader.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	var tests int
	for _, def := range doc.Packages {
		if def.TestPackage {
			tests++
		}
	}
	if tests == 0 {
		t.Error("-include-tests wrote a document with no test package in it")
	}
}

// TestNoImplicitInput covers a splint.json sitting beside a tree: it used to
// be read instead of parsing, so a job that extracted twice got the first
// document back and the second run reported a tree nobody had read.
func TestNoImplicitInput(t *testing.T) {
	root := t.TempDir()
	tree := writeTree(t, root, "tree")

	// A document describing nothing. A run that reads it reports nothing.
	empty := model.NewDocumentRoot(tree, "astparser")
	if err := loader.Save(filepath.Join(tree, saveFile), empty); err != nil {
		t.Fatal(err)
	}

	got := runIn(t, root, "-i", "tree", "--linters", "pairing", ".")
	if !strings.Contains(got, "tiny.go") {
		t.Errorf("a run beside a %s did not read the tree:\n%s", saveFile, got)
	}
}

// tiny is a module of one package, written where a test asks for it. The
// fixture module is read from where it sits and never written to; a test about
// where a document lands needs a tree it owns.
const tinyModule = `module example.com/tiny

go 1.27.0
`

// tinySource is one documented function, on lines a coverage profile can name.
const tinySource = `// Package tiny is one package with one function in it.
package tiny

// Do returns one.
func Do() int {
	return 1
}
`

// writeTree writes the tiny module into a new directory under root and returns
// the path to it.
func writeTree(t *testing.T, root, name string) string {
	t.Helper()

	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for file, content := range map[string]string{
		"go.mod":  tinyModule,
		"tiny.go": tinySource,
	} {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// runIn runs the command with the process in dir, the way a shell would run it
// there, and returns what it wrote.
func runIn(t *testing.T, dir string, args ...string) string {
	t.Helper()

	t.Chdir(dir)

	var out bytes.Buffer
	if _, err := run(context.Background(), args, &out, &out); err != nil {
		t.Fatalf("run(%v) error = %v", args, err)
	}
	return out.String()
}

// TestSaveWithASourcePath covers -save and -i together. The parse moves the
// process into the tree it reads, and the output path was joined onto the
// source path a second time on the way back out: "-i tree -save ." wrote
// tree/tree/splint.json, and the directory it named does not exist.
func TestSaveWithASourcePath(t *testing.T) {
	root := t.TempDir()
	tree := writeTree(t, root, "tree")

	runIn(t, root, "-i", "tree", "--save", ".")

	if _, err := os.Stat(filepath.Join(tree, saveFile)); err != nil {
		t.Errorf("-save wrote no %s beside the tree: %v", saveFile, err)
	}
	if _, err := os.Stat(filepath.Join(tree, "tree")); err == nil {
		t.Error("-save resolved the source path twice")
	}
}

// TestSaveRunsNoLinter covers --save as the extract: the run is there for the
// document it writes, so no linter runs and a tree the linters would report
// exits clean.
func TestSaveRunsNoLinter(t *testing.T) {
	root := t.TempDir()
	tree := writeTree(t, root, "tree")
	t.Chdir(tree)

	var out bytes.Buffer
	code, err := run(context.Background(), []string{"--save", "."}, &out, &out)
	if err != nil {
		t.Fatalf("run(--save) error = %v", err)
	}
	if code != exitClean {
		t.Errorf("run(--save) exit = %d, want %d: %s", code, exitClean, out.String())
	}
	if _, err := os.Stat(filepath.Join(tree, saveFile)); err != nil {
		t.Errorf("--save wrote no %s: %v", saveFile, err)
	}
}

// TestOutputWithASourcePath covers the other flag naming a file: -output is
// relative to the directory the command was run in, whichever tree it reads.
func TestOutputWithASourcePath(t *testing.T) {
	root := t.TempDir()
	tree := writeTree(t, root, "tree")

	runIn(t, root, "-i", "tree", "--output", "doc.json", ".")

	if _, err := os.Stat(filepath.Join(root, "doc.json")); err != nil {
		t.Errorf("-output wrote nothing where it was told: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tree, "doc.json")); err == nil {
		t.Error("-output wrote into the tree it read rather than into the working directory")
	}

	// The same path names the same file on the way back in.
	doc, err := loader.Load(filepath.Join(root, "doc.json"))
	if err != nil {
		t.Fatalf("the written document does not load: %v", err)
	}
	if len(doc.Packages) != 1 {
		t.Errorf("the written document holds %d packages, want 1", len(doc.Packages))
	}

	runIn(t, root, "--input", "doc.json")
}

// countedModule is the tiny module with a type, a method and a test, so the
// symbol rendering has something reached and something not.
var countedModule = map[string]string{
	"go.mod": `module example.com/tiny

go 1.27.0
`,
	"tiny.go": `// Package tiny is one package with a counter in it.
package tiny

// Counter counts.
type Counter struct{ n int }

// NewCounter builds a counter.
func NewCounter() *Counter { return &Counter{} }

// Add counts n in.
func (c *Counter) Add(n int) { c.n += n }

// Reset starts the count over.
func (c *Counter) Reset() { c.n = 0 }
`,
	"tiny_test.go": `package tiny

import "testing"

func TestAdd(t *testing.T) {
	c := NewCounter()
	c.Add(1)
}
`,
}

// writeCounted writes the module into a new directory under root.
func writeCounted(t *testing.T, root string) {
	t.Helper()

	for name, content := range countedModule {
		path := filepath.Join(root, "tree", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestDocsSymbol covers the symbol rendering: the declaration godoc style,
// and how many tests reach it.
func TestDocsSymbol(t *testing.T) {
	root := t.TempDir()
	writeCounted(t, root)

	out := runIn(t, root, "docs", "-i", "tree", ".", "Counter.Add")

	for _, want := range []string{
		`package tiny // import "example.com/tiny"`,
		"func (*Counter) Add",
		"Add counts n in.",
		"Tested by 1 test.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the symbol page is missing %q:\n%s", want, out)
		}
	}
}

// TestDocsSymbolUntested covers the count at zero: a method nothing reaches
// says so.
func TestDocsSymbolUntested(t *testing.T) {
	root := t.TempDir()
	writeCounted(t, root)

	out := runIn(t, root, "docs", "-i", "tree", ".", "Counter.Reset")

	if !strings.Contains(out, "Tested by 0 tests.") {
		t.Errorf("an unreached method does not say so:\n%s", out)
	}
}

// TestDocsSymbolVerbose covers the reference listing: one line per reference,
// the tests marked as tests, at the position of the referencing declaration.
func TestDocsSymbolVerbose(t *testing.T) {
	root := t.TempDir()
	writeCounted(t, root)

	out := runIn(t, root, "docs", "-v", "-i", "tree", ".", "NewCounter")

	for _, want := range []string{"References:", "test", "TestAdd", "tiny_test.go:5"} {
		if !strings.Contains(out, want) {
			t.Errorf("the verbose page is missing %q:\n%s", want, out)
		}
	}
}

// TestDocsSymbolMissing covers the error: a symbol the document does not hold
// is said to be missing rather than rendered as nothing.
func TestDocsSymbolMissing(t *testing.T) {
	root := t.TempDir()
	writeCounted(t, root)
	t.Chdir(root)

	var out bytes.Buffer
	_, err := run(context.Background(), []string{"docs", "-i", "tree", ".", "Nowhere"}, &out, &out)
	if err == nil || !strings.Contains(err.Error(), "Nowhere") {
		t.Errorf("run() error = %v, want the symbol named", err)
	}
}
