package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestHelpMarkdown covers the page a document holds and a program reads: every
// flag the parser defines is in it, which is what makes the page complete by
// construction rather than by somebody remembering to add one.
func TestHelpMarkdown(t *testing.T) {
	cfg, err := parseOptions(nil)
	if err != nil {
		t.Fatalf("parseOptions() error = %v", err)
	}

	var out bytes.Buffer
	if err := writeHelp(&out, helpSpec(cfg)); err != nil {
		t.Fatalf("writeHelp() error = %v", err)
	}
	page := out.String()

	for _, want := range []string{"# splint", "## Usage", "## Flags", "## Examples"} {
		if !strings.Contains(page, want) {
			t.Errorf("the page has no %q section:\n%s", want, page)
		}
	}

	for _, one := range flagDocs(cfg.flags) {
		if !strings.Contains(page, "`"+one.spelled()+"`") {
			t.Errorf("the page does not document %s:\n%s", one.spelled(), page)
		}
	}

	// A buffer is not a terminal, so the page carries no escape codes.
	if strings.Contains(page, "\033") {
		t.Error("the markdown page carries escape codes")
	}
}

// TestHelpTerminal covers the page a person reads: the same sections, painted.
func TestHelpTerminal(t *testing.T) {
	cfg, err := parseOptions(nil)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := helpTerminal(&out, helpSpec(cfg)); err != nil {
		t.Fatalf("helpTerminal() error = %v", err)
	}
	page := out.String()

	for _, want := range []string{
		helpSection + "splint" + helpReset,
		helpSection + "Usage:" + helpReset,
		helpSection + "Flags:" + helpReset,
		helpName + "-save" + helpReset,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not paint %q:\n%q", want, page)
		}
	}
}
