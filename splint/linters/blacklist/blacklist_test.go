package blacklist_test

import (
	"context"
	"testing"

	"github.com/titpetric/tools/splint/linters/blacklist"
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
func lint(t *testing.T, linter *blacklist.Linter, root *model.DocumentRoot) []model.Issue {
	t.Helper()

	report, err := linter.Lint(context.Background(), root)
	if err != nil {
		t.Fatalf("Lint() error = %v", err)
	}
	if report.Linter() != blacklist.Name {
		t.Errorf("Linter() = %q, want %q", report.Linter(), blacklist.Name)
	}
	issues := model.Issues(report)
	if len(issues) != report.Len() {
		t.Errorf("Len() = %d, walked %d", report.Len(), len(issues))
	}
	return issues
}

func TestLinter_Name(t *testing.T) {
	if name := blacklist.New().Name(); name != blacklist.Name {
		t.Errorf("Name() = %q, want %q", name, blacklist.Name)
	}
}

// TestLinter_Lint covers the host from both sides and the two shapes an
// import literal takes: a path alone and a path under an alias.
func TestLinter_Lint(t *testing.T) {
	tests := []struct {
		title       string
		literal     string
		blocked     bool
		replacement string
	}{
		{title: "a path", literal: `"gopkg.in/yaml.v2"`, blocked: true, replacement: "github.com/goccy/go-yaml"},
		{title: "an aliased path", literal: `yaml "gopkg.in/yaml.v3"`, blocked: true, replacement: "github.com/goccy/go-yaml"},
		{title: "a test assertion library", literal: `"gopkg.in/check.v1"`, blocked: true, replacement: "github.com/stretchr/testify"},
		{title: "a versioned path nothing replaces", literal: `"gopkg.in/thing.v1"`, blocked: true},
		{title: "a path below the host", literal: `"gopkg.in/thing.v1/sub"`, blocked: true},
		// The specific yaml.v3 block stops at a segment, and the host block
		// still covers the path, without a replacement.
		{title: "a name the specific block only prefixes", literal: `"gopkg.in/yaml.v30"`, blocked: true},
		{title: "the replacement itself", literal: `"github.com/goccy/go-yaml"`, blocked: false},
	}

	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			issues := lint(t, blacklist.New(), document(model.StringSet{"config.go": {test.literal}}))

			if !test.blocked {
				if len(issues) != 0 {
					t.Fatalf("reported %d issues for %s", len(issues), test.literal)
				}
				return
			}
			if len(issues) != 1 {
				t.Fatalf("reported %d issues for %s, want 1", len(issues), test.literal)
			}

			issue := issues[0]
			if issue.Rule != blacklist.RuleBlocked {
				t.Errorf("rule = %q, want %q", issue.Rule, blacklist.RuleBlocked)
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

// TestLinter_BlockOrder pins that the first block covering a path applies, so
// a specific replacement sits above the host entry it carves out of.
func TestLinter_BlockOrder(t *testing.T) {
	linter := &blacklist.Linter{Blocks: []blacklist.Block{
		{Prefix: "gopkg.in/yaml.v3", Replacement: "github.com/goccy/go-yaml"},
		{Prefix: "gopkg.in"},
	}}

	issues := lint(t, linter, document(model.StringSet{"config.go": {`"gopkg.in/yaml.v3"`}}))
	if len(issues) != 1 {
		t.Fatalf("reported %d issues, want 1", len(issues))
	}
	if got, _ := issues[0].Attr("replacement"); got != "github.com/goccy/go-yaml" {
		t.Errorf("replacement = %q, want the specific block", got)
	}
}

// TestLinter_Files pins that each import is reported against the file that
// writes it, and an import of another package stays out of the report.
func TestLinter_Files(t *testing.T) {
	root := document(model.StringSet{
		"config.go": {`yaml "gopkg.in/yaml.v3"`},
		"load.go":   {`"fmt"`, `"gopkg.in/check.v1"`},
	})

	issues := lint(t, blacklist.New(), root)
	if len(issues) != 2 {
		t.Fatalf("reported %d issues, want 2", len(issues))
	}
	for _, issue := range issues {
		if issue.Position.File != "config/"+map[string]string{"gopkg.in/yaml.v3": "config.go", "gopkg.in/check.v1": "load.go"}[issue.Symbol] {
			t.Errorf("%s reported against %s", issue.Symbol, issue.Position.File)
		}
	}
}

// TestLinter_Nothing pins that a tree importing nothing blocked reports
// nothing, and reads an empty document without complaint.
func TestLinter_Nothing(t *testing.T) {
	for _, root := range []*model.DocumentRoot{
		{},
		document(model.StringSet{"config.go": {`"fmt"`, `"encoding/json"`}}),
	} {
		if issues := lint(t, blacklist.New(), root); len(issues) != 0 {
			t.Errorf("reported %d issues, want none", len(issues))
		}
	}
}
