package checker

import (
	"slices"
	"testing"
)

func TestExpand(t *testing.T) {
	root := tree(t, map[string]string{
		"modules/login/dao/index.php":  "class login_dao {",
		"modules/login/index.php":      "class mod_login {",
		"modules/login/mod_admin.php":  "class mod_admin {",
		"modules/threads/index.php":    "class mod_threads {",
		"modules/threads/schema/a.sql": "",
		"theme/index.php":              "",
	})

	for _, one := range []struct {
		pattern string
		base    string
		want    []string
	}{
		{pattern: "", base: ".", want: []string{"."}},
		{pattern: "modules/{module}", base: ".", want: []string{"modules/login", "modules/threads"}},
		{pattern: "modules/*/index.php", base: ".", want: []string{"modules/login/index.php", "modules/threads/index.php"}},
		{pattern: "mod_{submodule}.php", base: "modules/login", want: []string{"modules/login/mod_admin.php"}},
		{pattern: "dao/", base: "modules/login", want: []string{"modules/login/dao"}},
		{pattern: "index.php/", base: "modules/login"},
		{pattern: "schema/*.sql", base: "modules/login"},
		{pattern: "missing/{module}", base: "."},
	} {
		var got []string
		for _, m := range expand(root, one.base, one.pattern, nil) {
			got = append(got, m.path)
		}
		slices.Sort(got)
		if !slices.Equal(got, one.want) {
			t.Errorf("%q under %q: got %v, want %v", one.pattern, one.base, got, one.want)
		}
	}
}

func TestExpandBinds(t *testing.T) {
	root := tree(t, map[string]string{
		"modules/login/dao/index.php":   "",
		"modules/threads/dao/index.php": "",
	})

	matches := expand(root, ".", "modules/{module}/dao", nil)
	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}
	for _, one := range matches {
		want := "modules/" + one.binds["module"] + "/dao"
		if one.path != want || !one.dir {
			t.Errorf("%+v binds %v", one, one.binds)
		}
	}

	inner := expand(root, matches[0].path, "index.php", matches[0].binds)
	if len(inner) != 1 || inner[0].binds["module"] != matches[0].binds["module"] {
		t.Errorf("a binding of a parent is lost: %+v", inner)
	}
}

func TestFree(t *testing.T) {
	for _, one := range []struct {
		pattern string
		binds   map[string]string
		want    bool
	}{
		{pattern: "index.php"},
		{pattern: "schema/*.up.sql"},
		{pattern: "mod_{submodule}.php", want: true},
		{pattern: "mod_{submodule}.php", binds: map[string]string{"submodule": "admin"}},
		{pattern: "{module}/{sub}", binds: map[string]string{"module": "login"}, want: true},
	} {
		if got := free(one.pattern, one.binds); got != one.want {
			t.Errorf("free(%q, %v) = %v", one.pattern, one.binds, got)
		}
	}
}

func TestSubst(t *testing.T) {
	binds := map[string]string{"module": "login"}
	for _, one := range []struct {
		text string
		want string
	}{
		{text: "class mod_{module} {", want: "class mod_login {"},
		{text: "class {module}_dao {", want: "class login_dao {"},
		{text: "class mod_{submodule} {", want: "class mod_{submodule} {"},
		{text: "function get*", want: "function get*"},
	} {
		if got := subst(one.text, binds); got != one.want {
			t.Errorf("subst(%q) = %q, want %q", one.text, got, one.want)
		}
	}
}

func TestHolds(t *testing.T) {
	content := "<?php\nclass mod_login {\n  function getUser() {}\n}\n"
	for _, one := range []struct {
		text string
		want bool
	}{
		{text: "class mod_login {", want: true},
		{text: "function get*", want: true},
		{text: "function set*"},
		{text: "mod_login*getUser"},
		{text: "@route"},
	} {
		if got := holds(content, one.text); got != one.want {
			t.Errorf("holds(%q) = %v", one.text, got)
		}
	}
}

func TestSegRE(t *testing.T) {
	re, names := segRE("mod_{submodule}.php")
	got := re.FindStringSubmatch("mod_admin.php")
	if len(names) != 1 || names[0] != "submodule" || got == nil || got[1] != "admin" {
		t.Fatalf("names %v, match %v", names, got)
	}
	if re.MatchString("mod_admin.phpx") || re.MatchString("index.php") {
		t.Error("the pattern matched past its ends")
	}
	if re, _ := segRE("*.up.sql"); !re.MatchString("001.up.sql") || re.MatchString("001.down.sql") {
		t.Error("a star segment matches the wrong names")
	}
}

func TestBind(t *testing.T) {
	binds := map[string]string{"module": "login"}
	got := bind(binds, []string{"submodule"}, []string{"admin"})
	if got["module"] != "login" || got["submodule"] != "admin" {
		t.Fatalf("got %v", got)
	}
	if _, ok := binds["submodule"]; ok {
		t.Error("a capture wrote into the binding of a parent")
	}
	if same := bind(binds, nil, nil); len(same) != 1 {
		t.Errorf("got %v", same)
	}
}

func TestWildcard(t *testing.T) {
	if got := wildcard("get*(", `[^\n]*`); got != `get[^\n]*\(` {
		t.Fatalf("got %q", got)
	}
}
