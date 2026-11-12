package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// conf checks a module for an index.php that names its class, and lints it.
const conf = `ruleslint {
  name: Modules

  dir modules/{module} {
    index.php {
      require 'class mod_{module} {'
    }
  }

  linters {
    echo linted
  }
}
`

func TestRun(t *testing.T) {
	root := site(t, map[string]string{
		"modules/login/index.php": "<?php class mod_login {}",
		".ruleslint.conf":         conf,
	})

	var out, errs bytes.Buffer
	code, err := run([]string{filepath.Join(root, ".ruleslint.conf")}, &out, &errs)
	if err != nil || code != 0 {
		t.Fatalf("code %d, err %v, out %q", code, err, out.String())
	}
	if !strings.Contains(out.String(), "Modules: 3 checks, 0 failed") {
		t.Errorf("got %q", out.String())
	}
}

func TestRunWritesEveryFailureThenExits1(t *testing.T) {
	root := site(t, map[string]string{
		"modules/login/index.php":   "<?php class mod_admin {}",
		"modules/threads/other.php": "",
		".ruleslint.conf":           conf,
	})
	file := filepath.Join(root, ".ruleslint.conf")

	var out, errs bytes.Buffer
	code, err := run([]string{file}, &out, &errs)
	if err != nil || code != 1 {
		t.Fatalf("code %d, err %v", code, err)
	}
	for _, want := range []string{
		"modules/login/index.php: does not hold 'class mod_login {'",
		"modules/threads: missing index.php",
		"modules/threads/other.php: no rule names it",
		"Modules: 5 checks, 3 failed",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out.String())
		}
	}
}

func TestRunWritesWhatALinterWritesToStderr(t *testing.T) {
	root := site(t, map[string]string{
		"modules/login/index.php": "<?php class mod_login {}",
		".ruleslint.conf":         conf,
	})

	var out, errs bytes.Buffer
	if _, err := run([]string{filepath.Join(root, ".ruleslint.conf")}, &out, &errs); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errs.String(), "linted") {
		t.Errorf("stderr %q", errs.String())
	}
	if strings.Contains(out.String(), "linted") {
		t.Errorf("a linter wrote into the report:\n%s", out.String())
	}
}

func TestRunExits2OnARuleFileThatDoesNotParse(t *testing.T) {
	root := site(t, map[string]string{".ruleslint.conf": "ruleslint {\n  dir a {\n"})

	var out, errs bytes.Buffer
	code, err := run([]string{filepath.Join(root, ".ruleslint.conf")}, &out, &errs)
	if err != nil || code != 2 {
		t.Fatalf("code %d, err %v", code, err)
	}
	if !strings.Contains(errs.String(), ".ruleslint.conf:2: dir block is never closed") {
		t.Errorf("got %q", errs.String())
	}
}

func TestRunChecksTheDirectoryOfTheRuleFile(t *testing.T) {
	root := site(t, map[string]string{
		"site/modules/login/index.php": "<?php class mod_login {}",
		"site/php.conf":                conf,
	})

	var out, errs bytes.Buffer
	code, err := run([]string{filepath.Join(root, "site/php.conf")}, &out, &errs)
	if err != nil || code != 0 {
		t.Fatalf("code %d, err %v, out %q", code, err, out.String())
	}
}

func TestRunRefusesTwoRuleFiles(t *testing.T) {
	var out, errs bytes.Buffer
	if _, err := run([]string{"a.conf", "b.conf"}, &out, &errs); err == nil {
		t.Error("two rule files were taken")
	}
	if _, err := run([]string{"-z"}, &out, &errs); err == nil {
		t.Error("an unknown flag was taken")
	}
}

func TestRunWritesTheHelpPage(t *testing.T) {
	var out, errs bytes.Buffer
	code, err := run([]string{"-h"}, &out, &errs)
	if err != nil || code != 0 {
		t.Fatalf("code %d, err %v", code, err)
	}
	if !strings.Contains(out.String(), "## Usage") {
		t.Errorf("got %q", out.String())
	}
}

// site writes the files of a fixture under a temporary root.
func site(t *testing.T, files map[string]string) string {
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
