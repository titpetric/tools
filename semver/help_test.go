package main

import (
	"flag"
	"strings"
	"testing"
)

// TestHelpMarkdown renders the page for a reader that is not a terminal, which
// is the markdown one, and looks for the parts a reader looks for.
func TestHelpMarkdown(t *testing.T) {
	page := helpSpec(flag.NewFlagSet("semver", flag.ContinueOnError))

	out := &strings.Builder{}
	if err := writeHelp(out, page); err != nil {
		t.Fatalf("writeHelp: %v", err)
	}

	for _, want := range []string{"## Usage", "## Examples", page.Tagline} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help page is missing %q:\n%s", want, out)
		}
	}
}
