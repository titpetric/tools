package main

import (
	"strings"
	"testing"

	"github.com/titpetric/tools/ruleslint/model"
)

func TestReport(t *testing.T) {
	checks := model.Checks{
		{Line: 16, Path: "modules/threads", Rule: "schema/*.up.sql", Detail: "missing schema/*.up.sql"},
		{Line: 20, Path: "modules/login/dao", Rule: "dao/", OK: true},
		{Line: 42, Rule: "phpscript lint ./...", Detail: "phpscript lint ./...: exit status 1"},
	}

	plain := &strings.Builder{}
	report(plain, ".ruleslint.conf", "TaskUI php app structure", checks)
	for _, want := range []string{
		".ruleslint.conf:16: modules/threads: missing schema/*.up.sql\n",
		".ruleslint.conf:42: phpscript lint ./...: exit status 1\n",
		"TaskUI php app structure: 3 checks, 2 failed\n",
	} {
		if !strings.Contains(plain.String(), want) {
			t.Errorf("missing %q in:\n%s", want, plain)
		}
	}
	if strings.Contains(plain.String(), "modules/login/dao") {
		t.Errorf("a passing check was written:\n%s", plain)
	}

	unnamed := &strings.Builder{}
	report(unnamed, ".ruleslint.conf", "", checks)
	if !strings.Contains(unnamed.String(), "ruleslint: 3 checks, 2 failed\n") {
		t.Errorf("an unnamed file is not named after the tool:\n%s", unnamed)
	}
}

func TestLine(t *testing.T) {
	one := model.Check{Line: 7, Path: "theme", Detail: "no such directory"}
	if got := line(one, "rules.conf", one.Detail); got != "rules.conf:7: theme: no such directory" {
		t.Fatalf("got %q", got)
	}
}
