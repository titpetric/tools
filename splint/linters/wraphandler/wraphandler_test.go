package wraphandler_test

import (
	"context"
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/linters/wraphandler"
	"github.com/titpetric/tools/splint/model"
)

// handler is a declaration of the http.HandlerFunc shape, which is the only
// signature the linter has anything to say about.
func handler(name, receiver, file string, line int) *model.Declaration {
	return &model.Declaration{
		Kind:      model.FuncKind,
		Name:      name,
		Receiver:  receiver,
		File:      file,
		Line:      line,
		Arguments: []string{"http.ResponseWriter", "*http.Request"},
	}
}

// wrapper is the counterpart a handler is meant to be a thin cover over.
func wrapper(name, receiver, file string, line int) *model.Declaration {
	decl := handler(name, receiver, file, line)
	decl.Returns = []string{"error"}
	return decl
}

// document wraps a list of functions into the document a linter reads.
func document(pkg model.Package, funcs model.DeclarationList, files model.FileList) *model.DocumentRoot {
	return &model.DocumentRoot{Packages: model.DefinitionList{{
		Package: pkg,
		Files:   files,
		Funcs:   funcs,
	}}}
}

// lint runs the linter and returns the report and its issues, checking on the
// way what every report owes the framework.
func lint(t *testing.T, root *model.DocumentRoot) (model.LintReport, []model.Issue) {
	t.Helper()

	report, err := wraphandler.New().Lint(context.Background(), root)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if report.Linter() != wraphandler.Name {
		t.Errorf("Linter() = %q, want %q", report.Linter(), wraphandler.Name)
	}

	issues := model.Issues(report)
	if len(issues) != report.Len() {
		t.Errorf("Len() = %d, walked %d", report.Len(), len(issues))
	}
	return report, issues
}

// TestLint reads the fixture under testdata: one handler wrapping nothing, one
// wrapping the unexported function below it.
func TestLint(t *testing.T) {
	pkg := model.Package{Package: "handler", ImportPath: "example.com/fixture/handler", Path: "."}

	root := document(pkg, model.DeclarationList{
		handler("Serve", "", "handler.go", 9),
		handler("Wrapped", "", "handler.go", 15),
		wrapper("wrapped", "", "handler.go", 22),
	}, model.FileList{{Name: "handler.go"}})

	report, issues := lint(t, root)
	if len(issues) != 1 {
		t.Fatalf("reported %d issues, want 1", len(issues))
	}
	if issues[0].Symbol != "Serve" {
		t.Errorf("symbol = %q, want %q", issues[0].Symbol, "Serve")
	}
	if issues[0].Rule != wraphandler.RuleUnwrapped {
		t.Errorf("rule = %q, want %q", issues[0].Rule, wraphandler.RuleUnwrapped)
	}
	if issues[0].Position.Ref() != "handler.go:9" {
		t.Errorf("position = %q, want %q", issues[0].Position.Ref(), "handler.go:9")
	}

	metric, known := report.Metrics().Packages["example.com/fixture/handler"].(wraphandler.Metric)
	if !known {
		t.Fatalf("metrics hold nothing for example.com/fixture/handler")
	}
	if metric.Handlers != 2 || metric.Wrapped != 1 {
		t.Errorf("metric = %+v, want 2 handlers and 1 wrapped", metric)
	}

	stats := report.Statistics()
	if len(stats) != 1 || len(stats[0].Rows) != 1 {
		t.Fatalf("statistics = %+v, want one table of one row", stats)
	}
	if row := stats[0].Rows[0]; row[1] != "2" || row[2] != "1" || row[3] != "50.0%" {
		t.Errorf("row = %v, want 2 handlers, 1 wrapped and a 50.0%% share", row)
	}
	if stats[0].Header == "" || stats[0].Footer == "" {
		t.Errorf("table reads without a header or a footer: %+v", stats[0])
	}
}

// TestLint_Method covers the counterpart of a method, which has to sit on the
// same receiver: another type's serve is not the work behind this one.
func TestLint_Method(t *testing.T) {
	pkg := model.Package{Package: "api", ImportPath: "example.com/api"}

	root := document(pkg, model.DeclarationList{
		handler("Serve", "*Server", "server.go", 10),
		wrapper("serve", "*Server", "server.go", 16),
		handler("Serve", "*Proxy", "proxy.go", 10),
		wrapper("serve", "*Gateway", "gateway.go", 16),
	}, nil)

	_, issues := lint(t, root)
	if len(issues) != 1 {
		t.Fatalf("reported %d issues, want 1", len(issues))
	}
	if issues[0].Symbol != "Proxy.Serve" {
		t.Errorf("symbol = %q, want %q", issues[0].Symbol, "Proxy.Serve")
	}
	if want := "Proxy.serve(w, r) error"; !strings.Contains(issues[0].Message, want) {
		t.Errorf("message = %q, want it to name %q", issues[0].Message, want)
	}
}

// TestLint_Signature covers the declarations the linter reads past. Only a
// function of the http.HandlerFunc shape is untestable, and only an exported
// one is a surface a consumer mounts, so none of these is counted either.
func TestLint_Signature(t *testing.T) {
	pkg := model.Package{Package: "api", ImportPath: "example.com/api"}

	returnsError := handler("Serve", "", "api.go", 3)
	returnsError.Returns = []string{"error"}

	oneArgument := handler("Serve", "", "api.go", 3)
	oneArgument.Arguments = []string{"http.ResponseWriter"}

	otherArguments := handler("Serve", "", "api.go", 3)
	otherArguments.Arguments = []string{"context.Context", "*http.Request"}

	notAFunc := handler("Serve", "", "api.go", 3)
	notAFunc.Kind = model.TypeKind

	tests := []struct {
		title string
		decl  *model.Declaration
	}{
		{title: "unexported", decl: handler("serve", "", "api.go", 3)},
		{title: "returns an error of its own", decl: returnsError},
		{title: "takes one argument", decl: oneArgument},
		{title: "takes a context rather than a writer", decl: otherArguments},
		{title: "is not a function", decl: notAFunc},
	}

	for _, test := range tests {
		report, issues := lint(t, document(pkg, model.DeclarationList{test.decl}, nil))
		if len(issues) != 0 {
			t.Errorf("%s: reported %q", test.title, issues[0].Message)
		}
		for _, stats := range report.Statistics() {
			if !stats.Empty() {
				t.Errorf("%s: counted as a handler", test.title)
			}
		}
	}
}

// TestLint_Skipped covers the scopes the convention is not a promise in: a
// file a generator wrote, a test file, and a test package.
func TestLint_Skipped(t *testing.T) {
	pkg := model.Package{Package: "api", ImportPath: "example.com/api"}

	generated := document(pkg,
		model.DeclarationList{handler("Serve", "", "api_gen.go", 3)},
		model.FileList{{Name: "api_gen.go", Generated: true}})
	if _, issues := lint(t, generated); len(issues) != 0 {
		t.Errorf("reported %d issues in a generated file", len(issues))
	}

	tests := document(pkg, model.DeclarationList{handler("Serve", "", "api_test.go", 3)}, nil)
	if _, issues := lint(t, tests); len(issues) != 0 {
		t.Errorf("reported %d issues in a test file", len(issues))
	}

	external := model.Package{Package: "api_test", ImportPath: "example.com/api_test", TestPackage: true}
	if _, issues := lint(t, document(external, model.DeclarationList{handler("Serve", "", "api.go", 3)}, nil)); len(issues) != 0 {
		t.Errorf("reported %d issues in a test package", len(issues))
	}

	// A counterpart only the test binary can see is not a counterpart: the
	// handler ships without it.
	shadowed := document(pkg, model.DeclarationList{
		handler("Serve", "", "api.go", 3),
		wrapper("serve", "", "api_test.go", 9),
	}, nil)
	if _, issues := lint(t, shadowed); len(issues) != 1 {
		t.Errorf("reported %d issues against a wrapper only a test can see, want 1", len(issues))
	}
}

// TestLint_Cancelled covers a document the linter is asked to stop reading,
// which is the linter failing rather than the code failing.
func TestLint_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	root := document(model.Package{Package: "api"}, model.DeclarationList{handler("Serve", "", "api.go", 3)}, nil)
	if _, err := wraphandler.New().Lint(ctx, root); err == nil {
		t.Errorf("Lint() error = nil, want the cancellation")
	}
}

func TestName(t *testing.T) {
	if got := wraphandler.New().Name(); got != wraphandler.Name {
		t.Errorf("Name() = %q, want %q", got, wraphandler.Name)
	}
}
