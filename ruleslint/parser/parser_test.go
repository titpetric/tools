package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// example is the rule file of the README, the shape the parser is written for.
const example = `ruleslint {
  name: TaskUI php app structure

  dir {
    modules
    theme
  }

  dir modules {
    login/
    threads/
    ...
  }

  dir modules/{module} {
    dao/
    ?schema/*.up.sql

    ?api/*.php {
      require '@route'
      deny 'class '
    }

    index.php {
      require 'class mod_{module} {'
      require 'function get*'
    }
  }

  linters {
    phpscript lint ./...
  }
}
`

func TestParse(t *testing.T) {
	cfg, err := Parse("example.conf", []byte(example))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Name != "TaskUI php app structure" {
		t.Errorf("name %q", cfg.Name)
	}
	if len(cfg.Dirs) != 3 {
		t.Fatalf("%d dirs, want 3", len(cfg.Dirs))
	}

	root := cfg.Dirs[0]
	if root.Pattern != "" || len(root.Entries) != 2 || root.Entries[0].Pattern != "modules" {
		t.Errorf("root dir %+v", root)
	}
	if got := cfg.Dirs[1]; got.Pattern != "modules" || len(got.Entries) != 2 || !got.Rest {
		t.Errorf("dir modules %+v, want two entries and the rest taken", got)
	}

	module := cfg.Dirs[2]
	if module.Pattern != "modules/{module}" || len(module.Entries) != 4 {
		t.Fatalf("dir %+v", module)
	}
	if schema := module.Entries[1]; schema.Pattern != "schema/*.up.sql" || !schema.Optional {
		t.Errorf("entry %+v, want the pattern of an optional path", schema)
	}
	api := module.Entries[2]
	if api.Pattern != "api/*.php" || !api.Optional || len(api.Rules) != 2 {
		t.Fatalf("entry %+v", api)
	}
	if api.Rules[0].Deny || !api.Rules[1].Deny {
		t.Errorf("require and deny read as %+v", api.Rules)
	}

	index := module.Entries[3]
	if index.Pattern != "index.php" || len(index.Rules) != 2 {
		t.Fatalf("entry %+v", index)
	}
	if index.Rules[0].Text != "class mod_{module} {" {
		t.Errorf("a quoted brace opened a block: %q", index.Rules[0].Text)
	}
	if index.Rules[0].Line != 25 {
		t.Errorf("require line %d, want 25", index.Rules[0].Line)
	}

	if len(cfg.Linters) != 1 || cfg.Linters[0].Command != "phpscript lint ./..." {
		t.Errorf("linters %+v", cfg.Linters)
	}
}

func TestParseFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".ruleslint.conf")
	if err := os.WriteFile(path, []byte("ruleslint {\n  name: X # a comment\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "X" {
		t.Errorf("name %q, want the comment dropped", cfg.Name)
	}
	if _, err := ParseFile(filepath.Join(t.TempDir(), "absent.conf")); err == nil {
		t.Error("an absent file parsed")
	}
}

func TestParseReportsEveryFault(t *testing.T) {
	src := `ruleslint {
  naem: X
  dirs modules {
    login/
  }
  dir theme {
    index.php {
      holds 'x'
      require: 'y'
    }
  }
}
}
ruleslint {
`
	_, err := Parse("bad.conf", []byte(src))
	if err == nil {
		t.Fatal("parsed a broken file")
	}
	for _, want := range []string{
		`bad.conf:2: unknown key "naem" in a ruleslint block`,
		`bad.conf:3: unknown block "dirs modules"`,
		`bad.conf:8: an entry block takes "require '<text>'" and "deny '<text>'" only, got "holds 'x'"`,
		`bad.conf:9: "require:" takes no colon, write "require '<text>'"`,
		`bad.conf:13: "}" closes no block`,
		`bad.conf:14: ruleslint block is never closed`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in:\n%s", want, err)
		}
	}
}

func TestParseWantsTheRuleslintBlock(t *testing.T) {
	_, err := Parse("bad.conf", []byte("rules {\n}\n"))
	if err == nil || !strings.Contains(err.Error(), `want "ruleslint {"`) {
		t.Fatalf("got %v", err)
	}
	_, err = Parse("bad.conf", []byte("modules\n"))
	if err == nil || !strings.Contains(err.Error(), "stands outside a block") {
		t.Fatalf("got %v", err)
	}
}

func TestParseRefusesALineAnEntryDoesNotTake(t *testing.T) {
	_, err := Parse("bad.conf", []byte("ruleslint {\n  dir a {\n    b.php {\n      c.php\n    }\n  }\n}\n"))
	if err == nil || !strings.Contains(err.Error(), `an entry block takes "require '<text>'"`) {
		t.Fatalf("got %v", err)
	}
	_, err = Parse("bad.conf", []byte("ruleslint {\n  dir a {\n    b.php {\n      require x\n    }\n  }\n}\n"))
	if err == nil || !strings.Contains(err.Error(), `got "require x"`) {
		t.Fatalf("unquoted text parsed: %v", err)
	}
}

func TestParseRefusesARuleOutsideAPathBlock(t *testing.T) {
	_, err := Parse("bad.conf", []byte("ruleslint {\n  dir a {\n    require 'x'\n  }\n}\n"))
	if err == nil || !strings.Contains(err.Error(), `bad.conf:3: "require" stands in a block under a path`) {
		t.Fatalf("got %v", err)
	}
}

func TestParseReadsTheOptionalPrefix(t *testing.T) {
	cfg, err := Parse("opt.conf", []byte("ruleslint {\n  dir ?a {\n    ?b.php\n    c.php\n  }\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	dir := cfg.Dirs[0]
	if dir.Pattern != "a" || !dir.Optional {
		t.Errorf("dir %+v, want the pattern of an optional directory", dir)
	}
	if got := dir.Entries[0]; got.Pattern != "b.php" || !got.Optional {
		t.Errorf("entry %+v, want an optional path", got)
	}
	if got := dir.Entries[1]; got.Pattern != "c.php" || got.Optional {
		t.Errorf("entry %+v, want a required path", got)
	}
}

func TestDirective(t *testing.T) {
	for _, one := range []struct {
		line  string
		key   string
		value string
		ok    bool
	}{
		{line: "name: TaskUI php app", key: "name", value: "TaskUI php app", ok: true},
		{line: `require: 'new Task\Module'`, key: "require", value: `new Task\Module`, ok: true},
		{line: `deny: "@route"`, key: "deny", value: "@route", ok: true},
		{line: "require:", key: "require", ok: true},
		{line: "schema/*.up.sql"},
		{line: "c:/win/path.php"},
	} {
		key, value, ok := directive(one.line)
		if ok != one.ok || key != one.key || value != one.value {
			t.Errorf("%q: got %q %q %v", one.line, key, value, ok)
		}
	}
}

func TestStatement(t *testing.T) {
	for _, one := range []struct {
		line string
		stmt string
		text string
		ok   bool
	}{
		{line: `require '@route'`, stmt: "require", text: "@route", ok: true},
		{line: `deny "class "`, stmt: "deny", text: "class ", ok: true},
		{line: `require	'class mod_login {'`, stmt: "require", text: "class mod_login {", ok: true},
		{line: `require ''`, stmt: "require", ok: true},
		{line: `require: '@route'`},
		{line: "require @route"},
		{line: "require"},
		{line: `'@route'`},
	} {
		stmt, text, ok := statement(one.line)
		if ok != one.ok || stmt != one.stmt || text != one.text {
			t.Errorf("%q: got %q %q %v", one.line, stmt, text, ok)
		}
	}
}
