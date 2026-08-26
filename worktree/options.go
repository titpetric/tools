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
	Resolve    bool
	Verdict    bool
	Chain      bool
	Stats      bool
	Apply      bool
	GoVersion  string
	Release    string
	From       string
	To         string
	FilterPath string
	FilterArg  string
	Skipped    int
}

// commandConfig opens the setup screen instead of scanning the workspace.
const commandConfig = "config"

// commandResolve releases the selected modules in dependency order.
const commandResolve = "resolve"

// commandVerdict reports the release one module has earned, as markdown.
const commandVerdict = "verdict"

// valueFlags lists the flags that take a value as a separate argument.
var valueFlags = map[string]bool{"-go": true, "--go": true, "-from": true, "--from": true, "-to": true, "--to": true}

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
	flag.BoolVar(&opts.All, "all", false, "include all modules (default: skip modules without releases/changes); with verdict, report every release in the chain")
	flag.BoolVar(&opts.PUML, "puml", false, "output PlantUML dependency diagram to stdout")
	flag.BoolVar(&opts.D2, "d2", false, "output D2 dependency diagram to stdout")
	flag.BoolVar(&opts.Matrix, "t", false, "output dependency matrix to stdout")
	flag.BoolVar(&opts.Verbose, "v", false, "verbose output: show module details, every untracked file, and commands run during updates")
	flag.BoolVar(&opts.Verbose, "verbose", false, "alias of -v")
	flag.BoolVar(&opts.Apply, "apply", false, "perform the resolution instead of rendering it")
	flag.BoolVar(&opts.Stats, "stats", false, "collapse worktree verdict to a table of counts, one row per release")
	flag.StringVar(&opts.From, "from", "", "the revision worktree verdict measures from, a tag by default; all, 0 or HEAD report every release")
	flag.StringVar(&opts.To, "to", "", "the revision worktree verdict measures to, the working tree by default")
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

	// Resolve subcommands. The release and setup screens take no path filter;
	// "resolve" and "verdict" do, so they read theirs from the argument after
	// them.
	filter := 0
	if flag.NArg() > 0 {
		switch flag.Arg(0) {
		case releasePatch, releaseMinor:
			opts.Release = flag.Arg(0)
			return opts
		case commandConfig:
			opts.Configure = true
			return opts
		case commandVerdict:
			opts.Verdict = true
			opts.setChain()
			opts.setFilter(flag.Arg(1))
			return opts
		case commandResolve:
			opts.Resolve = true
			filter = 1
		}
	}

	opts.setFilter(flag.Arg(filter))
	return opts
}

// chainKeywords are the values of --from that ask for every release rather than
// name one revision. They all say the same thing, which is "start at the
// beginning": a version below every tag, or the whole history the repository
// standing at HEAD has behind it.
var chainKeywords = map[string]bool{"all": true, "0": true, "0.0.0": true, "v0.0.0": true, "head": true}

// chainKeyword reports whether a --from value asks for the whole release chain.
func chainKeyword(from string) bool {
	return chainKeywords[strings.ToLower(strings.TrimSpace(from))]
}

// setChain records that the verdict covers every release rather than one range,
// which --all asks for outright and --from asks for with a keyword. The keyword
// is cleared once it is read, so nothing downstream is handed a version that
// names no revision.
func (o *Options) setChain() {
	if chainKeyword(o.From) {
		o.Chain, o.From = true, ""
	}
	if o.All {
		o.Chain = true
	}
}

// setFilter records the optional path filter, which selects the modules a
// command works on. An argument naming an existing directory is resolved to an
// absolute path so it can be matched against the module directories, and
// "./..." means the whole workspace, which is no filter at all.
func (o *Options) setFilter(arg string) {
	if arg == "" || arg == "./..." {
		return
	}

	o.FilterArg = arg
	if abs, err := filepath.Abs(arg); err == nil {
		if _, err := os.Stat(abs); err == nil {
			o.FilterPath = abs
		}
	}
	if o.FilterPath == "" {
		o.FilterPath = o.FilterArg
	}
}
