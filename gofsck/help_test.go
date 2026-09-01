package main

import (
	"flag"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHelp renders the page a pipe reads, and asks it for the headings and for
// every flag the parser defines: the page is written from the parser, so a
// flag added later is documented by having been defined.
func TestHelp(t *testing.T) {
	out := &strings.Builder{}
	assert.NoError(t, writeHelp(out, helpSpec(flag.CommandLine)))

	page := out.String()

	for _, heading := range []string{"## Usage", "## Flags", "## Examples"} {
		assert.Contains(t, page, heading, "the page is missing a heading")
	}

	flag.CommandLine.VisitAll(func(one *flag.Flag) {
		assert.Contains(t, page, "`-"+one.Name, "the page is missing a flag")
	})
}
