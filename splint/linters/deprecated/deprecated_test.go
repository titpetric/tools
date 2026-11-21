package deprecated_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/titpetric/tools/splint/linters/deprecated"
	"github.com/titpetric/tools/splint/model"
)

// write puts the source of a fixture in the tree.
func write(t *testing.T, root, name, source string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

// definition is one package as the parser records it.
func definition(pkg model.Package, imports model.StringSet, funcs model.DeclarationList) *model.Definition {
	return &model.Definition{
		Package: pkg,
		Imports: imports,
		Files:   model.FileList{{Name: "x.go"}},
		Funcs:   funcs,
	}
}

// lint runs the linter and returns the issues it found.
func lint(t *testing.T, root *model.DocumentRoot) []model.Issue {
	t.Helper()
	report, err := deprecated.New().Lint(context.Background(), root)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if report.Linter() != deprecated.Name {
		t.Errorf("Linter() = %q, want %q", report.Linter(), deprecated.Name)
	}
	issues := model.Issues(report)
	if len(issues) != report.Len() {
		t.Errorf("Len() = %d, walked %d", report.Len(), len(issues))
	}
	if report.Metrics().Packages != nil && len(report.Metrics().Packages) > 0 {
		t.Errorf("Metrics() = %v, want none", report.Metrics())
	}
	if report.Statistics() != nil {
		t.Errorf("Statistics() = %v, want none", report.Statistics())
	}
	return issues
}

// A call from another package and one from the package itself both name the
// deprecated function, at the line each is written on.
func TestLinter_Lint(t *testing.T) {
	root := t.TempDir()
	write(t, root, "lib/lib.go", `package lib

// Old does it the old way.
//
// Deprecated: use New.
func Old() {}

func New() {}

// Caller still calls Old.
func Caller() { Old() }
`)
	write(t, root, "app/app.go", `package app

import "example.com/lib"

func Run() {
	lib.Old()
}
`)

	document := &model.DocumentRoot{Root: root, Packages: model.DefinitionList{
		definition(
			model.Package{Package: "lib", ImportPath: "example.com/lib", Path: "./lib"},
			nil,
			model.DeclarationList{
				{Kind: model.FuncKind, Name: "Old", File: "lib.go", Line: 6, Doc: "Old does it the old way.\n\nDeprecated: use New."},
				{Kind: model.FuncKind, Name: "New", File: "lib.go", Line: 8},
				{Kind: model.FuncKind, Name: "Caller", File: "lib.go", Line: 11, Globals: model.StringSet{"Old": nil}},
			},
		),
		definition(
			model.Package{Package: "app", ImportPath: "example.com/app", Path: "./app"},
			model.StringSet{"app.go": {`"example.com/lib"`}},
			model.DeclarationList{
				{Kind: model.FuncKind, Name: "Run", File: "app.go", Line: 5, References: model.StringSet{"lib": {"Old"}}},
			},
		),
	}}

	issues := lint(t, document)
	if len(issues) != 2 {
		t.Fatalf("reported %d issues, want 2: %#v", len(issues), issues)
	}
	if issues[0].Position.Ref() != "app/app.go:6" {
		t.Errorf("first use = %q, want app/app.go:6", issues[0].Position.Ref())
	}
	if issues[1].Position.Ref() != "lib/lib.go:11" {
		t.Errorf("second use = %q, want lib/lib.go:11", issues[1].Position.Ref())
	}
	if issues[0].Severity != model.SeverityError {
		t.Errorf("severity = %v, want error", issues[0].Severity)
	}
	if issues[0].Message != "uses deprecated Old: use New." {
		t.Errorf("message = %q", issues[0].Message)
	}
}

// A document with no Deprecated paragraph reports nothing.
func TestLinter_Clean(t *testing.T) {
	document := &model.DocumentRoot{Packages: model.DefinitionList{
		definition(
			model.Package{Package: "app", ImportPath: "example.com/app", Path: "./app"},
			nil,
			model.DeclarationList{
				{Kind: model.FuncKind, Name: "Run", File: "app.go", Line: 5},
			},
		),
	}}

	if issues := lint(t, document); len(issues) != 0 {
		t.Fatalf("reported %d issues, want none: %#v", len(issues), issues)
	}
}
