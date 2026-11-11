package main

import (
	"flag"
	"strings"
	"testing"
)

func TestHelpMarkdown(t *testing.T) {
	page := helpSpec(flag.NewFlagSet("ruleslint", flag.ContinueOnError))
	out := &strings.Builder{}
	if err := writeHelp(out, page); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Usage", "## Examples", page.Tagline} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q", want)
		}
	}
}
