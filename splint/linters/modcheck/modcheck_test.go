package modcheck_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/linters/modcheck"
	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/modproxy"
)

// lint runs the linter with no proxy, so the coupling is measured and nothing
// is asked of the network.
func lint(t *testing.T, root *model.DocumentRoot) model.LintReport {
	t.Helper()

	report, err := (&modcheck.Linter{}).Lint(context.Background(), root)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	return report
}

// rules is how many findings there are of each rule.
func rules(report model.LintReport) map[string]int {
	out := map[string]int{}
	for issue := range report.All() {
		out[issue.Rule]++
	}
	return out
}

// document is a module requiring four things and importing two of them.
func document() *model.DocumentRoot {
	return &model.DocumentRoot{
		Modules: []*model.Module{{
			Path: "example.com/main",
			Requires: []model.Require{
				{Path: "example.com/deep", Version: "v1.0.0"},
				{Path: "example.com/thin", Version: "v0.2.0"},
				{Path: "example.com/unused", Version: "v0.1.0"},
				{Path: "example.com/hidden", Version: "v3.0.0", Indirect: true},
			},
		}},
		Packages: model.DefinitionList{
			{
				Package: model.Package{Package: "a", ImportPath: "example.com/main/a", Path: "./a"},
				Imports: model.StringSet{
					"a.go": {`"example.com/deep"`, `"example.com/thin"`},
				},
				Funcs: model.DeclarationList{{
					Kind: model.FuncKind, Name: "Do", File: "a.go",
					References: model.StringSet{
						"deep": {"One", "Two", "Three"},
						"thin": {"Only"},
					},
				}},
			},
			{
				Package: model.Package{Package: "b", ImportPath: "example.com/main/b", Path: "./b"},
				Imports: model.StringSet{"b.go": {`"example.com/deep"`}},
				Funcs: model.DeclarationList{{
					Kind: model.FuncKind, Name: "Go", File: "b.go",
					References: model.StringSet{"deep": {"Four"}},
				}},
			},
		},
	}
}

// TestLinter_Lint covers the counts, which are what say whether a dependency
// can be removed: files is the blast radius, packages the spread, and symbols
// how much of it is used.
func TestLinter_Lint(t *testing.T) {
	metrics := lint(t, document()).Metrics()

	deep, ok := metrics.Packages["example.com/deep"].(modcheck.Dependency)
	if !ok {
		t.Fatalf("metrics hold %v", metrics.PackageKeys())
	}
	if deep.Files != 2 || deep.Packages != 2 || deep.Symbols != 4 {
		t.Errorf("deep = %d files, %d pkgs, %d symbols, want 2, 2, 4", deep.Files, deep.Packages, deep.Symbols)
	}
	if deep.Kind() != "direct" || !deep.Shipped() {
		t.Errorf("deep reads %q, shipped %v", deep.Kind(), deep.Shipped())
	}

	unused := metrics.Packages["example.com/unused"].(modcheck.Dependency)
	if unused.Files != 0 || unused.Kind() != "unused" {
		t.Errorf("unused = %d files, %q", unused.Files, unused.Kind())
	}
	if unused.Shipped() {
		t.Error("a dependency nothing imports is not linked by a consumer")
	}
}

func TestLinter_LintRules(t *testing.T) {
	got := rules(lint(t, document()))

	// One import nothing reaches, and one reached from one file through one
	// symbol. The indirect requirement is nobody's choice and is not reported.
	if got[modcheck.RuleUnused] != 1 {
		t.Errorf("unused = %d, want 1: %v", got[modcheck.RuleUnused], got)
	}
	if got[modcheck.RuleThin] != 1 {
		t.Errorf("thin = %d, want 1: %v", got[modcheck.RuleThin], got)
	}
	if got[modcheck.RuleReplace] != 0 || got[modcheck.RuleMajors] != 0 {
		t.Errorf("reported a replace or a major split that is not there: %v", got)
	}
}

// TestLinter_LintReplace covers the one rule that fails a run: what a replace
// builds against is not what the go.mod requires.
func TestLinter_LintReplace(t *testing.T) {
	root := document()
	root.Modules[0].Replaces = []model.Replace{{Path: "example.com/deep", NewPath: "../deep"}}

	report := lint(t, root)
	if got := rules(report)[modcheck.RuleReplace]; got != 1 {
		t.Fatalf("replace = %d, want 1", got)
	}

	for issue := range report.All() {
		if issue.Rule != modcheck.RuleReplace {
			continue
		}
		if issue.Severity != model.SeverityError {
			t.Errorf("a replace reports at %v, want error: a run has to fail on it", issue.Severity)
		}
		// A module is not in a file, so the finding names the go.mod, which is
		// where a reader goes to act on it.
		if issue.Position.Ref() != "go.mod" {
			t.Errorf("position = %q", issue.Position.Ref())
		}
		if !strings.Contains(issue.Message, "../deep") {
			t.Errorf("message does not say what it resolves to: %q", issue.Message)
		}
	}

	deep := report.Metrics().Packages["example.com/deep"].(modcheck.Dependency)
	if deep.Kind() != "replaced" {
		t.Errorf("deep reads %q, want replaced", deep.Kind())
	}
}

func TestLinter_LintMajors(t *testing.T) {
	root := document()
	root.Modules[0].Requires = append(root.Modules[0].Requires,
		model.Require{Path: "example.com/deep/v2", Version: "v2.0.0"})

	if got := rules(lint(t, root))[modcheck.RuleMajors]; got != 1 {
		t.Errorf("majors = %d, want 1", got)
	}
}

// TestLinter_LintTestOnly covers a dependency a consumer never links, which is
// a different thing from one that is used.
func TestLinter_LintTestOnly(t *testing.T) {
	root := &model.DocumentRoot{
		Modules: []*model.Module{{
			Path:     "example.com/main",
			Requires: []model.Require{{Path: "example.com/testing", Version: "v1.0.0"}},
		}},
		Packages: model.DefinitionList{{
			Package: model.Package{Package: "a_test", ImportPath: "example.com/main/a_test", Path: "./a", TestPackage: true},
			Imports: model.StringSet{"a_test.go": {`"example.com/testing"`}},
		}},
	}

	dep := lint(t, root).Metrics().Packages["example.com/testing"].(modcheck.Dependency)
	if dep.Kind() != "test only" {
		t.Errorf("kind = %q, want test only", dep.Kind())
	}
	if dep.Shipped() {
		t.Error("a test only dependency is not linked by a consumer")
	}
}

// TestLinter_LintDriverImport covers the import that reaches no symbol: a
// driver is imported for what it registers, and there is nothing to inline.
func TestLinter_LintDriverImport(t *testing.T) {
	root := &model.DocumentRoot{
		Modules: []*model.Module{{
			Path:     "example.com/main",
			Requires: []model.Require{{Path: "example.com/driver", Version: "v1.0.0"}},
		}},
		Packages: model.DefinitionList{{
			Package: model.Package{Package: "a", ImportPath: "example.com/main/a", Path: "./a"},
			Imports: model.StringSet{"a.go": {`"example.com/driver"`}},
		}},
	}

	if got := rules(lint(t, root))[modcheck.RuleThin]; got != 0 {
		t.Errorf("thin = %d, want 0 for an import that reaches no symbol", got)
	}
}

// blanked is a module blank importing four packages: one from the file that
// wires the program, one from the file that wires the test binary, one from a
// package of its own, and one from a library file.
func blanked() *model.DocumentRoot {
	return &model.DocumentRoot{
		Modules: []*model.Module{{
			Path:     "example.com/main",
			Requires: []model.Require{{Path: "example.com/driver", Version: "v1.0.0"}},
		}},
		Packages: model.DefinitionList{
			{
				Package: model.Package{Package: "main", ImportPath: "example.com/main", Path: "."},
				Imports: model.StringSet{
					"main.go":      {`_ "example.com/driver"`},
					"main_test.go": {`_ "example.com/driver"`},
				},
			},
			{
				Package: model.Package{Package: "a", ImportPath: "example.com/main/a", Path: "./a"},
				Imports: model.StringSet{
					"a.go": {
						`_ "example.com/driver"`,
						`_ "example.com/main/registry"`,
						`_ "embed"`,
						`"fmt"`,
					},
				},
			},
		},
	}
}

// TestLinter_LintBlank covers where a blank import is allowed to be: a package
// imported for its init alone decides what the binary does, and main.go and
// main_test.go are where that is decided.
func TestLinter_LintBlank(t *testing.T) {
	report := lint(t, blanked())

	if got := rules(report)[modcheck.RuleBlank]; got != 1 {
		t.Fatalf("blank = %d, want 1: only the library file is reported", got)
	}

	for issue := range report.All() {
		if issue.Rule != modcheck.RuleBlank {
			continue
		}
		if issue.Severity != model.SeverityWarn {
			t.Errorf("a blank import reports at %v, want warn", issue.Severity)
		}
		// The finding is in the file that writes the import, not in the go.mod
		// the rest of this linter reports against.
		if issue.Position.Ref() != "a/a.go" {
			t.Errorf("position = %q, want a/a.go", issue.Position.Ref())
		}
		if issue.Symbol != "example.com/driver" {
			t.Errorf("symbol = %q", issue.Symbol)
		}
	}
}

// fakeProxy answers the four questions a lookup asks. Every module weighs a
// thousand bytes, and the one named deep requires thin, so the reach of a
// dependency is something a test can count.
//
// The client it returns caches nowhere: a test does not write to the cache the
// machine it runs on keeps.
func fakeProxy(t *testing.T) *modproxy.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodHead:
			w.Header().Set("Content-Length", "1000")
			w.WriteHeader(http.StatusOK)

		case strings.HasSuffix(r.URL.Path, ".mod"):
			if strings.Contains(r.URL.Path, "/deep/") {
				fmt.Fprint(w, "module example.com/deep\n\nrequire example.com/thin v0.2.0\n")
				return
			}
			fmt.Fprint(w, "module example.com/other\n")

		default:
			json.NewEncoder(w).Encode(map[string]any{"Version": "v1.0.0"})
		}
	}))
	t.Cleanup(server.Close)

	return &modproxy.Client{Proxy: server.URL, HTTP: server.Client(), Sizes: modproxy.OpenCache("")}
}

// TestLinter_LintReach covers what a dependency brings in behind it. A go.mod
// records what the build resolved to and not which requirement asked for what,
// so the graph is read one dependency's go.mod at a time.
func TestLinter_LintReach(t *testing.T) {
	linter := &modcheck.Linter{Proxy: fakeProxy(t)}

	report, err := linter.Lint(context.Background(), document())
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	metrics := report.Metrics()

	deep := metrics.Packages["example.com/deep"].(modcheck.Dependency)
	if deep.Size != 1000 {
		t.Fatalf("deep = %d bytes, want the Content-Length of the zip", deep.Size)
	}
	if deep.Reach != 1 || deep.Weight != 2000 {
		t.Errorf("deep reaches %d modules weighing %d, want 1 and 2000", deep.Reach, deep.Weight)
	}

	// A module requiring nothing weighs itself, and is reached by deep rather
	// than reaching anything.
	thin := metrics.Packages["example.com/thin"].(modcheck.Dependency)
	if thin.Reach != 0 || thin.Weight != 1000 {
		t.Errorf("thin reaches %d modules weighing %d, want 0 and 1000", thin.Reach, thin.Weight)
	}
}

func TestLinter_Statistics(t *testing.T) {
	stats := lint(t, document()).Statistics()
	if len(stats) != 1 {
		t.Fatalf("Statistics() = %d tables, want 1", len(stats))
	}

	table := stats[0]
	if table.Header == "" || table.Footer == "" {
		t.Errorf("table reads header %q and footer %q, want both", table.Header, table.Footer)
	}
	if len(table.Labels) != 10 || len(table.Rows) != 4 {
		t.Fatalf("Labels = %v, Rows = %d", table.Labels, len(table.Rows))
	}
	// The size column reads as nothing when nobody was asked, rather than as
	// a zero that looks measured.
	for _, row := range table.Rows {
		if row[2] != "-" {
			t.Errorf("size = %q with no proxy, want %q", row[2], "-")
		}
	}
	if !strings.Contains(table.Footer, "4 dependencies") {
		t.Errorf("footer = %q", table.Footer)
	}
}

func TestLinter_Name(t *testing.T) {
	if got := modcheck.New().Name(); got != modcheck.Name {
		t.Errorf("Name() = %q", got)
	}
}
