package model

import (
	"testing"
)

func TestChecks_Failed(t *testing.T) {
	checks := Checks{
		{Path: "modules/login/dao", Rule: "dao/", OK: true},
		{Path: "modules/login", Rule: "schema/*.up.sql", Detail: "missing schema/*.up.sql"},
		{Path: "modules/login/index.php", Rule: "require 'class mod_login {'", Detail: "no match"},
	}
	if got := checks.Failed(); got != 2 {
		t.Fatalf("failed %d, want 2", got)
	}
	if got := Checks(nil).Failed(); got != 0 {
		t.Fatalf("failed %d of nothing, want 0", got)
	}
}

func TestConfig(t *testing.T) {
	cfg := &Config{
		Name: "TaskUI php app structure",
		Dirs: []*Dir{
			{Line: 3, Entries: []*Entry{{Pattern: "modules", Line: 4}}},
			{Pattern: "modules/{module}", Line: 8, Entries: []*Entry{
				{Pattern: "index.php", Line: 9, Rules: []Rule{{Text: "class mod_{module} {", Line: 10}}},
			}},
		},
		Linters: []Linter{{Command: "phpscript lint ./...", Line: 14}},
	}

	if cfg.Dirs[0].Pattern != "" {
		t.Errorf("root dir pattern %q, want empty", cfg.Dirs[0].Pattern)
	}
	entry := cfg.Dirs[1].Entries[0]
	if entry.Rules[0].Text != "class mod_{module} {" {
		t.Errorf("rule %q", entry.Rules[0].Text)
	}
	if cfg.Linters[0].Command != "phpscript lint ./..." {
		t.Errorf("linter %q", cfg.Linters[0].Command)
	}
}

func TestRule_Key(t *testing.T) {
	for _, test := range []struct {
		rule Rule
		want string
	}{
		{rule: Rule{Text: "@route"}, want: "require"},
		{rule: Rule{Text: "class ", Deny: true}, want: "deny"},
	} {
		if got := test.rule.Key(); got != test.want {
			t.Errorf("key %q, want %q", got, test.want)
		}
	}
}

func TestCheck(t *testing.T) {
	one := Check{Line: 4, Path: "modules", Rule: "modules", Detail: "missing modules"}
	if one.OK {
		t.Fatalf("got %+v, want a failure", one)
	}
}
