package main

import (
	"flag"
	"strings"
	"testing"
)

func TestHelpSpec(t *testing.T) {
	fs := flag.NewFlagSet("ruleslint", flag.ContinueOnError)
	fs.Bool("v", false, "list every check, passed or not")

	page := helpSpec(fs)
	if page.Name != "ruleslint" || page.Tagline == "" || len(page.Usage) != 1 {
		t.Fatalf("got %+v", page)
	}
	for _, want := range []string{"dir <pattern>", "require '<text>'", "deny '<text>'", "linters", `"?"`, `"..."`} {
		if !strings.Contains(page.Description, want) {
			t.Errorf("the description does not name %q", want)
		}
	}
	if len(page.Examples) != 3 || !strings.Contains(page.Notes, "Exits 1") {
		t.Errorf("examples %+v, notes %q", page.Examples, page.Notes)
	}
}
