package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
)

// Options holds command-line options for worktree.
type Options struct {
	Update     bool
	All        bool
	PUML       bool
	D2         bool
	Verbose    bool
	FilterPath string
	FilterArg  string
	Skipped    int
}

// ParseOptions parses command-line flags and returns Options.
func ParseOptions() *Options {
	// Reorder os.Args so flags come before positional args,
	// allowing e.g. "worktree platform -v" to work.
	var flags, positional []string
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
		} else {
			positional = append(positional, arg)
		}
	}
	os.Args = append([]string{os.Args[0]}, append(flags, positional...)...)

	opts := &Options{}
	flag.BoolVar(&opts.Update, "u", false, "update workspace dependencies (tidy only, with --all: go get -u ./...)")
	flag.BoolVar(&opts.All, "all", false, "include all modules (default: skip modules without releases/changes)")
	flag.BoolVar(&opts.PUML, "puml", false, "output PlantUML dependency diagram to stdout")
	flag.BoolVar(&opts.D2, "d2", false, "output D2 dependency diagram to stdout")
	flag.BoolVar(&opts.Verbose, "v", false, "verbose output: show module column and dependency lists")
	flag.Parse()

	// Resolve optional path filter
	if flag.NArg() > 0 {
		opts.FilterArg = flag.Arg(0)
		abs, err := filepath.Abs(opts.FilterArg)
		if err == nil {
			if _, err := os.Stat(abs); err == nil {
				opts.FilterPath = abs
			}
		}
		if opts.FilterPath == "" {
			opts.FilterPath = opts.FilterArg
		}
	}

	return opts
}
