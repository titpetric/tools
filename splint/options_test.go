package main

import (
	"testing"
)

// TestDocsSymbolArguments covers how splint docs reads its operands: two are
// a pattern and a symbol, one is a symbol when it reads like one and a
// pattern when it does not.
func TestDocsSymbolArguments(t *testing.T) {
	tests := []struct {
		args    []string
		pattern string
		symbol  string
	}{
		{[]string{"docs", ".", "Counter.Add"}, ".", "Counter.Add"},
		{[]string{"docs", "./...", "Open"}, "./...", "Open"},
		{[]string{"docs", "Open"}, ".", "Open"},
		{[]string{"docs", "./..."}, "./...", ""},
		{[]string{"docs", "model"}, "model", ""},
		{[]string{"./...", "Open"}, "Open", ""},
	}

	for _, test := range tests {
		cfg, err := parseOptions(test.args)
		if err != nil {
			t.Fatalf("parseOptions(%v) error = %v", test.args, err)
		}
		if cfg.options.Pattern != test.pattern {
			t.Errorf("parseOptions(%v) pattern = %q, want %q", test.args, cfg.options.Pattern, test.pattern)
		}
		if cfg.docsSymbol != test.symbol {
			t.Errorf("parseOptions(%v) symbol = %q, want %q", test.args, cfg.docsSymbol, test.symbol)
		}
	}
}

// TestVerboseFlag covers the two spellings of one flag.
func TestVerboseFlag(t *testing.T) {
	for _, arg := range []string{"-v", "--verbose"} {
		cfg, err := parseOptions([]string{arg, "./..."})
		if err != nil {
			t.Fatalf("parseOptions(%s) error = %v", arg, err)
		}
		if !cfg.options.Verbose {
			t.Errorf("parseOptions(%s) did not set Verbose", arg)
		}
	}
}
