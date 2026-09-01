package main

import (
	"flag"
	"strings"
	"testing"
)

// TestHelpMarkdown reads the page a pipe gets: every flag the parser defines is
// on it, under the headings a reader looks them up by.
func TestHelpMarkdown(t *testing.T) {
	fs := flag.NewFlagSet("puzzle", flag.ContinueOnError)
	bindFlags(fs)

	page := &strings.Builder{}
	if err := writeHelp(page, helpSpec(fs)); err != nil {
		t.Fatalf("writeHelp: %v", err)
	}

	out := page.String()

	for _, heading := range []string{"## Usage", "## Flags", "## Examples"} {
		if !strings.Contains(out, heading) {
			t.Errorf("page has no %q heading", heading)
		}
	}

	fs.VisitAll(func(one *flag.Flag) {
		if !strings.Contains(out, "`-"+one.Name) {
			t.Errorf("page does not document -%s", one.Name)
		}
	})
}
