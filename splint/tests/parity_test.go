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
)

// workspace is where the projects the harness reads are checked out, which
// SPLINT_WORKSPACE overrides. A project that is not there is skipped, so the
// harness reads whatever the machine has rather than requiring all of them.
var workspace = envOr("SPLINT_WORKSPACE", "/root/workspace/github")

// envOr reads an environment variable, falling back to a default.
func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// projects are the trees to compare on. They are real repositories rather than
// fixtures because the point is the shapes nobody thought to write a fixture
// for: build tags, generic types, generated code, files that do not compile.
var projects = []string{
	"atkins", "cli", "etl", "exp", "go-web-crontab", "lessgo", "mig", "oida",
	"pdo", "phpscript", "platform", "platform-app", "secretsmanager", "tools",
	"vuego", "vuego-cli",
}

// budget is the share of declarations that may differ on a field nobody has
// explained yet. It is a ratchet: the number below is what the harness reports
// today, and lowering it as the differences are chased is the point of having
// it. Raising it is a decision somebody has to make on purpose.
const budget = 0.025

// allowed are the fields the two parsers are not expected to agree on, and
// why. A field not named here has to match, and the summary says so.
var allowed = map[string]string{
	"Complexity.Cognitive": "gocognit weights a branch by how deeply it nests in the tree, which a line scan has no tree to read",
	"Complexity.Cyclomatic": "gocyclo counts the branch nodes of a tree, and a line scan counts the keywords " +
		"that produce them, which part company inside a composite literal",
}

// TestParity runs the simple parser over every project and compares what it
// produced against what "go-fsck extract" produced for the same tree.
//
// go-fsck is the reference rather than the ast parser in this module, because
// go-fsck is what the model has to stay compatible with: a document the two
// agree on is one every tool already reading go-fsck output can read.
func TestParity(t *testing.T) {
	if _, err := exec.LookPath("go-fsck"); err != nil {
		t.Skip("go-fsck is not installed")
	}

	totals := map[string]int{}
	var declarations, matched int

	for _, project := range projects {
		root := filepath.Join(workspace, project)
		if _, err := os.Stat(root); err != nil {
			t.Logf("skipping %s, not checked out", project)
			continue
		}

		t.Run(project, func(t *testing.T) {
			counts, seen, agreed := compare(t, root)
			for field, count := range counts {
				totals[field] += count
			}
			declarations += seen
			matched += agreed
		})
	}

	if declarations == 0 {
		t.Skip("no project was read")
	}

	t.Logf("total: %d declarations, %d matched (%.1f%%)", declarations, matched, percent(matched, declarations))

	unexplained := 0
	for _, field := range sortedKeys(totals) {
		note, ok := allowed[field]
		if !ok {
			note = "unexplained"
			unexplained += totals[field]
		}
		t.Logf("  %-22s %6d  %s", field, totals[field], note)
	}

	share := float64(unexplained) / float64(declarations)
	t.Logf("unexplained: %d differences over %d declarations, %.2f%% of budget %.2f%%",
		unexplained, declarations, share*100, budget*100)
	if share > budget {
		t.Errorf("the parsers differ on %.2f%% of declarations, over the budget of %.2f%%", share*100, budget*100)
	}
}

// compare reads one project with both, and returns the per field difference
// counts, how many declarations there were and how many matched entirely.
func compare(t *testing.T, root string) (map[string]int, int, int) {
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

	var (
		matched    int
		counts     = map[string]int{}
		onlyRef    []string
		onlyParsed []string
	)

	for key, a := range left {
		b, ok := right[key]
		if !ok {
			onlyRef = append(onlyRef, key)
			continue
		}
		fields := diff(a, b)
		if len(fields) == 0 {
			matched++
			continue
		}
		for _, field := range fields {
			counts[field]++
		}
	}
	for key := range right {
		if _, ok := left[key]; !ok {
			onlyParsed = append(onlyParsed, key)
		}
	}

	sort.Strings(onlyRef)
	sort.Strings(onlyParsed)

	t.Logf("%d declarations, %d matched (%.1f%%), %d only in go-fsck, %d only in simple",
		len(left), matched, percent(matched, len(left)), len(onlyRef), len(onlyParsed))
	for _, field := range sortedKeys(counts) {
		note, ok := allowed[field]
		if !ok {
			note = "unexplained"
		}
		t.Logf("  %-22s %6d  %s", field, counts[field], note)
	}
	for _, key := range head(onlyRef, 3) {
		t.Logf("  only in go-fsck: %s", key)
	}
	for _, key := range head(onlyParsed, 3) {
		t.Logf("  only in simple:  %s", key)
	}

	counts["_onlyReference"] = len(onlyRef)
	counts["_onlyParsed"] = len(onlyParsed)
	return counts, len(left), matched
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

// index keys every declaration by package, kind, symbol and file, so two
// documents of the same tree compare entry by entry.
func index(doc *model.DocumentRoot) map[string]*model.Declaration {
	out := map[string]*model.Declaration{}

	for _, def := range doc.Packages {
		if def.TestPackage {
			continue
		}
		for _, decl := range def.DeclarationList() {
			key := fmt.Sprintf("%s|%s|%s|%s", def.Package.Path, decl.Kind, decl.Symbol(), decl.File)
			out[key] = decl
		}
	}

	return out
}

// diff names the fields two declarations disagree on.
func diff(a, b *model.Declaration) []string {
	var fields []string

	compareString := func(name, left, right string) {
		if strings.TrimSpace(left) != strings.TrimSpace(right) {
			fields = append(fields, name)
		}
	}
	compareList := func(name string, left, right []string) {
		if strings.Join(left, "\x00") != strings.Join(right, "\x00") {
			fields = append(fields, name)
		}
	}

	compareString("Type", a.Type, b.Type)
	compareString("Receiver", a.Receiver, b.Receiver)
	compareString("Signature", a.Signature, b.Signature)
	compareString("Doc", a.Doc, b.Doc)
	if alignment(a.Source) != alignment(b.Source) {
		fields = append(fields, "Source")
	}
	compareList("Arguments", a.Arguments, b.Arguments)
	compareList("Returns", a.Returns, b.Returns)
	compareList("Names", a.Names, b.Names)
	compareList("References", flatten(a.References), flatten(b.References))
	compareList("Fields", fieldStrings(a), fieldStrings(b))

	if a.Line != b.Line {
		fields = append(fields, "Line")
	}
	if a.SelfContained != b.SelfContained {
		fields = append(fields, "SelfContained")
	}
	if a.Complexity != nil && b.Complexity != nil {
		if a.Complexity.Cognitive != b.Complexity.Cognitive {
			fields = append(fields, "Complexity.Cognitive")
		}
		if a.Complexity.Cyclomatic != b.Complexity.Cyclomatic {
			fields = append(fields, "Complexity.Cyclomatic")
		}
		if a.Complexity.Lines != b.Complexity.Lines {
			fields = append(fields, "Complexity.Lines")
		}
	}

	return fields
}

// alignment normalises the column padding out of a source listing.
//
// go-fsck renders a declaration through go/printer, which writes the padding
// that lines up a run of struct fields or const values as tabs. The file on
// disk carries what gofmt wrote there, which is spaces. The two say the same
// thing and a comparison of the bytes does not, so the padding inside a line
// is collapsed before they are compared; the indentation that opens a line is
// left alone, since that is code structure and not alignment.
func alignment(source string) string {
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

// flatten renders a reference set as a sorted list, so two of them compare.
func flatten(set model.StringSet) []string {
	var out []string
	for _, key := range set.Keys() {
		for _, value := range set[key] {
			out = append(out, key+"."+value)
		}
	}
	sort.Strings(out)
	return out
}

// fieldStrings renders the fields of a declaration as a comparable list.
func fieldStrings(decl *model.Declaration) []string {
	out := make([]string, 0, len(decl.Fields))
	for _, field := range decl.Fields {
		out = append(out, fmt.Sprintf("%s %s `%s` json=%s", field.Name, field.Type, field.Tag, field.JSONName))
	}
	return out
}

func sortedKeys(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func head(list []string, n int) []string {
	if len(list) <= n {
		return list
	}
	return list[:n]
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
