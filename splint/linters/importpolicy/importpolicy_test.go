package importpolicy_test

import (
	"context"
	"testing"

	"github.com/titpetric/tools/splint/linters/importpolicy"
	"github.com/titpetric/tools/splint/model"
)

// document wraps the imports of one package into the document a linter reads.
func document(imports model.StringSet) *model.DocumentRoot {
	return &model.DocumentRoot{Packages: model.DefinitionList{{
		Package: model.Package{Package: "config", Path: "./config"},
		Imports: imports,
	}}}
}

// lint runs the linter and returns its issues.
func lint(t *testing.T, linter *importpolicy.Linter, root *model.DocumentRoot) []model.Issue {
	t.Helper()

	report, err := linter.Lint(context.Background(), root)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if report.Linter() != importpolicy.Name {
		t.Errorf("Linter() = %q, want %q", report.Linter(), importpolicy.Name)
	}
	issues := model.Issues(report)
	if len(issues) != report.Len() {
		t.Errorf("Len() = %d, walked %d", report.Len(), len(issues))
	}
	return issues
}

func TestLinter_Name(t *testing.T) {
	if name := importpolicy.New().Name(); name != importpolicy.Name {
		t.Errorf("Name() = %q, want %q", name, importpolicy.Name)
	}
}

// TestNew pins the linter a run with no tree setting gets: the default deny
// list and an allow list nobody wrote.
func TestNew(t *testing.T) {
	linter := importpolicy.New()
	if len(linter.DenyList) != len(importpolicy.DefaultDenyList) {
		t.Errorf("DenyList holds %d entries, want the %d defaults", len(linter.DenyList), len(importpolicy.DefaultDenyList))
	}
	if len(linter.AllowList) != 0 {
		t.Errorf("AllowList holds %v, want nothing", linter.AllowList)
	}

	linter.DenyList[0] = importpolicy.Deny{Prefix: "example.com"}
	if importpolicy.DefaultDenyList[0].Prefix == "example.com" {
		t.Error("the linter writes through to DefaultDenyList, which every other run reads")
	}
}

// TestLinter_Lint covers the host from both sides and the two shapes an
// import literal takes: a path alone and a path under an alias.
func TestLinter_Lint(t *testing.T) {
	tests := []struct {
		title       string
		literal     string
		denied      bool
		replacement string
	}{
		{title: "a path", literal: `"gopkg.in/yaml.v2"`, denied: true, replacement: "github.com/goccy/go-yaml"},
		{title: "an aliased path", literal: `yaml "gopkg.in/yaml.v3"`, denied: true, replacement: "github.com/goccy/go-yaml"},
		{title: "a test assertion library", literal: `"gopkg.in/check.v1"`, denied: true, replacement: "github.com/stretchr/testify"},
		{title: "a versioned path nothing replaces", literal: `"gopkg.in/thing.v1"`, denied: true},
		{title: "a path below the host", literal: `"gopkg.in/thing.v1/sub"`, denied: true},
		// The specific yaml.v3 entry stops at a segment, and the host entry
		// still covers the path, without a replacement.
		{title: "a name the specific entry only prefixes", literal: `"gopkg.in/yaml.v30"`, denied: true},
		{title: "the replacement itself", literal: `"github.com/goccy/go-yaml"`, denied: false},
	}

	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			issues := lint(t, importpolicy.New(), document(model.StringSet{"config.go": {test.literal}}))

			if !test.denied {
				if len(issues) != 0 {
					t.Fatalf("reported %d issues for %s", len(issues), test.literal)
				}
				return
			}
			if len(issues) != 1 {
				t.Fatalf("reported %d issues for %s, want 1", len(issues), test.literal)
			}

			issue := issues[0]
			if issue.Rule != importpolicy.RuleDenied {
				t.Errorf("rule = %q, want %q", issue.Rule, importpolicy.RuleDenied)
			}
			if issue.Severity != model.SeverityWarn {
				t.Errorf("severity = %v, want a warning", issue.Severity)
			}
			if issue.Position.File != "config/config.go" || issue.Position.Package != "config" {
				t.Errorf("position = %+v, want config/config.go of package config", issue.Position)
			}

			replacement, ok := issue.Attr("replacement")
			if test.replacement == "" {
				if ok {
					t.Errorf("replacement = %q, want none", replacement)
				}
				return
			}
			if replacement != test.replacement {
				t.Errorf("replacement = %q, want %q", replacement, test.replacement)
			}
		})
	}
}

// TestLinter_AllowList covers the list that beats the other: an allowed path
// stays where the deny list refuses the host around it, and the entry covers a
// directory rather than a name it prefixes.
func TestLinter_AllowList(t *testing.T) {
	linter := importpolicy.New()
	linter.AllowList = []string{"gopkg.in/yaml.v3"}

	tests := map[string]bool{
		`"gopkg.in/yaml.v3"`:        false,
		`"gopkg.in/yaml.v3/decode"`: false,
		`"gopkg.in/yaml.v30"`:       true,
		`"gopkg.in/yaml.v2"`:        true,
	}

	for literal, denied := range tests {
		issues := lint(t, linter, document(model.StringSet{"config.go": {literal}}))
		if denied && len(issues) != 1 {
			t.Errorf("%s reported %d issues, want 1", literal, len(issues))
		}
		if !denied && len(issues) != 0 {
			t.Errorf("%s reported %d issues, want none: the allow list covers it", literal, len(issues))
		}
	}
}

// TestLinter_DenyOrder pins that the first entry covering a path applies, so a
// specific replacement sits above the host entry it narrows.
func TestLinter_DenyOrder(t *testing.T) {
	linter := &importpolicy.Linter{DenyList: []importpolicy.Deny{
		{Prefix: "gopkg.in/yaml.v3", Replacement: "github.com/goccy/go-yaml"},
		{Prefix: "gopkg.in"},
	}}

	issues := lint(t, linter, document(model.StringSet{"config.go": {`"gopkg.in/yaml.v3"`}}))
	if len(issues) != 1 {
		t.Fatalf("reported %d issues, want 1", len(issues))
	}
	if got, _ := issues[0].Attr("replacement"); got != "github.com/goccy/go-yaml" {
		t.Errorf("replacement = %q, want the specific entry", got)
	}
}

// TestLinter_Files pins that each import is reported against the file that
// writes it, and an import of another package stays out of the report.
func TestLinter_Files(t *testing.T) {
	root := document(model.StringSet{
		"config.go": {`yaml "gopkg.in/yaml.v3"`},
		"load.go":   {`"fmt"`, `"gopkg.in/check.v1"`},
	})

	issues := lint(t, importpolicy.New(), root)
	if len(issues) != 2 {
		t.Fatalf("reported %d issues, want 2", len(issues))
	}
	for _, issue := range issues {
		if issue.Position.File != "config/"+map[string]string{"gopkg.in/yaml.v3": "config.go", "gopkg.in/check.v1": "load.go"}[issue.Symbol] {
			t.Errorf("%s reported against %s", issue.Symbol, issue.Position.File)
		}
	}
}

// TestLinter_Nothing pins that a tree importing nothing denied reports
// nothing, and reads an empty document without complaint.
func TestLinter_Nothing(t *testing.T) {
	for _, root := range []*model.DocumentRoot{
		{},
		document(model.StringSet{"config.go": {`"fmt"`, `"encoding/json"`}}),
	} {
		if issues := lint(t, importpolicy.New(), root); len(issues) != 0 {
			t.Errorf("reported %d issues, want none", len(issues))
		}
	}
}
