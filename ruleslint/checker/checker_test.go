package checker

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/titpetric/tools/ruleslint/model"
	"github.com/titpetric/tools/ruleslint/parser"
)

// rules is the rule file of the README, with the modules it names cut to the
// two the fixture tree holds.
const rules = `ruleslint {
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

  dir theme {
    dao/
    templates/
    templates_c/
    index.php
  }

  dir modules/{module} {
    dao/
    templates/
    templates_c/
    schema/*.up.sql

    api/*.php {
      require: '@route'
      deny: 'class '
    }

    modinfo.php {
      require: 'new Task\Module'
    }

    index.php {
      require: 'class mod_{module} {'
      require: 'function get*'
    }

    mod_{submodule}.php {
      require: 'class mod_{submodule} {'
    }
  }

  dir modules/{module}/dao {
    index.php {
      require: 'class {module}_dao {'
    }
  }
}
`

// site is a tree of two modules: login keeps every rule, threads breaks four.
var site = map[string]string{
	"modules/login/dao/index.php":       "<?php class login_dao {}",
	"modules/login/templates/.keep":     "",
	"modules/login/templates_c/.keep":   "",
	"modules/login/schema/001.up.sql":   "create table login (id text);",
	"modules/login/api/list.php":        "<?php // @route GET /login",
	"modules/login/modinfo.php":         `<?php new Task\Module('login');`,
	"modules/login/index.php":           "<?php class mod_login {\n function getUser() {}\n}",
	"modules/login/mod_admin.php":       "<?php class mod_admin {}",
	"modules/threads/dao/index.php":     "<?php class threads_dao {}",
	"modules/threads/templates/.keep":   "",
	"modules/threads/templates_c/.keep": "",
	"modules/threads/api/list.php":      "<?php class threads_api {}",
	"modules/threads/modinfo.php":       `<?php new Task\Module('threads');`,
	"modules/threads/index.php":         "<?php class mod_thread {}",
	"theme/dao/.keep":                   "",
	"theme/templates/.keep":             "",
	"theme/templates_c/.keep":           "",
	"theme/index.php":                   "<?php",
}

func TestRunner_Run(t *testing.T) {
	root := tree(t, site)
	checks, err := Runner{Root: root}.Run(parse(t, rules))
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"modules/threads: missing schema/*.up.sql",
		"modules/threads/api/list.php: does not hold '@route'",
		"modules/threads/api/list.php: holds 'class '",
		"modules/threads/index.php: does not hold 'class mod_threads {'",
		"modules/threads/index.php: does not hold 'function get*'",
	}
	if got := fails(checks); !slices.Equal(got, want) {
		t.Errorf("failures:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if checks.Failed() != len(want) {
		t.Errorf("failed %d of %d checks", checks.Failed(), len(checks))
	}
}

func TestRunner_RunBindsAPlaceholderPerModule(t *testing.T) {
	checks, err := Runner{Root: tree(t, site)}.Run(parse(t, rules))
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []model.Check{
		{Path: "modules/login/index.php", Rule: "require: 'class mod_login {'", OK: true},
		{Path: "modules/login/dao/index.php", Rule: "require: 'class login_dao {'", OK: true},
		{Path: "modules/login/mod_admin.php", Rule: "require: 'class mod_admin {'", OK: true},
	} {
		if !slices.ContainsFunc(checks, func(one model.Check) bool {
			return one.Path == want.Path && one.Rule == want.Rule && one.OK == want.OK
		}) {
			t.Errorf("no check %q of %s", want.Rule, want.Path)
		}
	}

	// threads has no mod_*.php, and an unbound placeholder requires none.
	for _, one := range checks {
		if strings.Contains(one.Rule, "{submodule}") {
			t.Errorf("an unbound placeholder was checked: %+v", one)
		}
	}
}

func TestRunner_RunNamesAMissingDirectory(t *testing.T) {
	root := tree(t, map[string]string{"theme": "a file, not a directory"})
	checks, err := Runner{Root: root}.Run(parse(t, "ruleslint {\n dir theme {\n }\n dir modules {\n }\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"theme: not a directory", "modules: no such directory"}
	if got := fails(checks); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRunner_RunRefusesTheTextOfADirectory(t *testing.T) {
	root := tree(t, map[string]string{"modules/login/dao/index.php": ""})
	src := "ruleslint {\n dir modules/login {\n  dao {\n   require: 'class'\n  }\n }\n}\n"

	checks, err := Runner{Root: root}.Run(parse(t, src))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"modules/login/dao: is a directory and holds no text"}
	if got := fails(checks); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRunner_RunRefusesARootThatIsNoTree(t *testing.T) {
	root := tree(t, map[string]string{"index.php": ""})
	if _, err := (Runner{Root: filepath.Join(root, "index.php")}).Run(parse(t, "ruleslint {\n}\n")); err == nil {
		t.Error("a file was checked as a tree")
	}
	if _, err := (Runner{Root: filepath.Join(root, "absent")}).Run(parse(t, "ruleslint {\n}\n")); err == nil {
		t.Error("an absent root was checked")
	}
}

func TestRunner(t *testing.T) {
	cfg := parse(t, "ruleslint {\n  linters {\n    echo linted\n    exit 3\n  }\n}\n")
	root := tree(t, map[string]string{"index.php": ""})

	out := &bytes.Buffer{}
	checks, err := Runner{Root: root, Out: out}.Run(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 2 || !checks[0].OK || checks[1].OK {
		t.Fatalf("got %+v", checks)
	}
	if !strings.Contains(checks[1].Detail, "exit status 3") {
		t.Errorf("detail %q", checks[1].Detail)
	}
	if !strings.Contains(out.String(), "linted") {
		t.Errorf("output %q", out.String())
	}

	quiet, err := Runner{Root: root}.Run(cfg)
	if err != nil || len(quiet) != 2 {
		t.Errorf("a linter needs no writer: %+v %v", quiet, err)
	}
}

// tree writes a fixture under a temporary root.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// parse reads the rule file a test writes, and fails the test when it does not.
func parse(t *testing.T, src string) *model.Config {
	t.Helper()

	cfg, err := parser.Parse("test.conf", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// fails lists the failing checks as the report writes them.
func fails(checks model.Checks) []string {
	var out []string
	for _, one := range checks {
		if !one.OK {
			out = append(out, one.Path+": "+one.Detail)
		}
	}
	return out
}
