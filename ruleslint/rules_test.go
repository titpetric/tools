package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "rules"), 0o755)
	os.MkdirAll(filepath.Join(root, "modules/a/storage"), 0o755)
	os.WriteFile(filepath.Join(root, "modules/a/index.php"), nil, 0o644)
	os.WriteFile(filepath.Join(root, "modules/a/migrate.php"), nil, 0o644)
	file := filepath.Join(root, "rules/m.yml")
	os.WriteFile(file, []byte(`name: Module
input:
  - dirs: modules/*
rules:
  - rule: require index.php
     message: a
  - rule: require dao or storage
    message: b
  - rule: require modinfo.php
    message: c
  - rule: error migrate.php
    message: d
`), 0o644)
	set, err := load(file, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	res, _ := set.check()
	if res.Required != 3 || res.Met != 2 || res.Fails != 2 || len(res.Modules) != 1 || res.Modules[0].Fails != 2 {
		t.Fatalf("got %+v", res)
	}
}

func TestLoadReportsEveryBadRule(t *testing.T) {
	file := filepath.Join(t.TempDir(), "rules", "m.yml")
	os.MkdirAll(filepath.Dir(file), 0o755)
	os.WriteFile(file, []byte("name: M\nrules:\n  - rule: bogus a\n    message: x\n  - rule: lonely\n    message: y\n  - rule: require ok\n    message: z\n"), 0o644)
	_, err := load(file, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "unknown kind") || !strings.Contains(err.Error(), "want") {
		t.Fatalf("got %v", err)
	}
}

func TestVerboseListsEveryCheck(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "rules"), 0o755)
	os.MkdirAll(filepath.Join(root, "modules/a"), 0o755)
	os.WriteFile(filepath.Join(root, "rules/m.yml"), []byte("name: Module\ninput:\n  - dirs: modules/*\nrules:\n  - rule: require x\n    message: need x\n  - rule: pass templates\n    message: t\n"), 0o644)
	set, _ := load(filepath.Join(root, "rules/m.yml"), io.Discard)
	res, _ := set.check()
	var plain, verbose strings.Builder
	res.write(&plain, set.Name, false)
	res.write(&verbose, set.Name, true)
	if strings.Contains(plain.String(), "miss pass") || !strings.Contains(verbose.String(), "miss pass templates") ||
		!strings.Contains(plain.String(), "a: required 0/1, passing 0/1, errors: 1") {
		t.Fatalf("plain:\n%s\nverbose:\n%s", plain.String(), verbose.String())
	}
}

func TestRunReportsEveryFailureThenExits1(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, "rules"), 0o755)
	for _, m := range []string{"a", "b", "c"} {
		os.MkdirAll(filepath.Join(d, "mods", m), 0o755)
	}
	rf := filepath.Join(d, "rules", "m.yml")
	os.WriteFile(rf, []byte("name: M\ninput:\n  - dirs: mods/*\n  - dirs: \"[\"\nrules:\n  - rule: require x.php\n    message: need x\n"), 0o644)
	var out, errb bytes.Buffer
	code, err := run([]string{rf}, &out, &errb)
	if err != nil || code != 1 {
		t.Fatalf("code %d err %v", code, err)
	}
	for _, m := range []string{"mods/a", "mods/b", "mods/c"} {
		if !strings.Contains(out.String(), "m "+m+": missing required x.php") {
			t.Errorf("missing %s in %q", m, out.String())
		}
	}
	if !strings.Contains(out.String(), "input") {
		t.Errorf("bad pattern not reported: %q", out.String())
	}
}

func TestFailureNamesWhatIsMissing(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "rules"), 0o755)
	os.MkdirAll(filepath.Join(root, "modules/a"), 0o755)
	os.WriteFile(filepath.Join(root, "modules/a/migrate.php"), nil, 0o644)
	file := filepath.Join(root, "rules/m.yml")
	os.WriteFile(file, []byte("name: Module\ninput:\n  - dirs: modules/*\nrules:\n  - rule: require dao or storage\n    message: b\n  - rule: error migrate.php\n    message: d\n"), 0o644)
	set, _ := load(file, io.Discard)
	res, _ := set.check()
	var buf bytes.Buffer
	res.write(&buf, "Module", false)
	for _, want := range []string{"missing required dao or storage, looked for " + filepath.Join(root, "modules/a/dao") + " or " + filepath.Join(root, "modules/a/storage"), "forbidden migrate.php is present at " + filepath.Join(root, "modules/a/migrate.php")} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("want %q in %s", want, buf.String())
		}
	}
}
