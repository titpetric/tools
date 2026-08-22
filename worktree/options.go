package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Options holds command-line options for worktree.
type Options struct {
	Update     bool
	UpdateAll  bool
	Pull       bool
	All        bool
	PUML       bool
	D2         bool
	Matrix     bool
	Verbose    bool
	Configure  bool
	GoVersion  string
	Release    string
	FilterPath string
	FilterArg  string
	Skipped    int
}

// commandConfig opens the setup screen instead of scanning the workspace.
const commandConfig = "config"

// valueFlags lists the flags that take a value as a separate argument.
var valueFlags = map[string]bool{"-go": true, "--go": true}

// ParseOptions parses command-line flags and returns Options.
func ParseOptions() *Options {
	// Reorder os.Args so flags come before positional args,
	// allowing e.g. "worktree platform -v" to work. A flag taking a value
	// keeps the argument after it, so "worktree --go 1.27 ./..." works.
	var flags, positional []string
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		flags = append(flags, arg)
		if valueFlags[arg] && i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	os.Args = append([]string{os.Args[0]}, append(flags, positional...)...)

	opts := &Options{}
	flag.BoolVar(&opts.Update, "u", false, "update the workspace dependencies that are behind their latest tag, and tidy")
	flag.BoolVar(&opts.UpdateAll, "U", false, "update every dependency with go get -u ./..., including ones outside the workspace")
	flag.BoolVar(&opts.Pull, "pull", false, "pull new changes for each git repository")
	flag.BoolVar(&opts.All, "all", false, "include all modules (default: skip modules without releases/changes)")
	flag.BoolVar(&opts.PUML, "puml", false, "output PlantUML dependency diagram to stdout")
	flag.BoolVar(&opts.D2, "d2", false, "output D2 dependency diagram to stdout")
	flag.BoolVar(&opts.Matrix, "t", false, "output dependency matrix to stdout")
	flag.BoolVar(&opts.Verbose, "v", false, "verbose output: show module details and commands run during updates")
	flag.StringVar(&opts.GoVersion, "go", "", "set the go directive of every go.mod and go.work to this version, then update dependencies")
	flag.Parse()

	// -U is a wider -u, so it implies it.
	if opts.UpdateAll {
		opts.Update = true
	}

	if opts.GoVersion != "" {
		goVersion, err := parseGoVersion(opts.GoVersion)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			flag.Usage()
			os.Exit(2)
		}
		opts.GoVersion = goVersion
	}

	// Resolve subcommands, which take no path filter
	if flag.NArg() > 0 {
		switch flag.Arg(0) {
		case releasePatch, releaseMinor:
			opts.Release = flag.Arg(0)
			return opts
		case commandConfig:
			opts.Configure = true
			return opts
		}
	}

	// Resolve optional path filter
	if flag.NArg() > 0 && flag.Arg(0) != "./..." {
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
