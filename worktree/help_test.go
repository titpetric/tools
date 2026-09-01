package main

import (
	"bytes"
	"flag"
	"io"
	"strings"
	"testing"
)

// TestHelpSpec checks the markdown page holds every flag the parser defines,
// spelled the way it is typed, and every heading a reader looks for.
func TestHelpSpec(t *testing.T) {
	fs := flag.NewFlagSet("worktree", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	(&Options{}).bind(fs)

	buf := &bytes.Buffer{}
	if err := writeHelp(buf, helpSpec(fs)); err != nil {
		t.Fatalf("writeHelp() error = %v", err)
	}
	page := buf.String()

	for _, heading := range []string{"## Usage", "## Commands", "## Flags", "## Examples"} {
		if !strings.Contains(page, heading) {
			t.Errorf("help page is missing the %q heading", heading)
		}
	}

	fs.VisitAll(func(one *flag.Flag) {
		placeholder, _ := flag.UnquoteUsage(one)
		want := "`-" + one.Name
		if placeholder != "" {
			want += " " + placeholder
		}
		want += "`"

		if !strings.Contains(page, want) {
			t.Errorf("help page is missing the flag %s", want)
		}
	})
}
