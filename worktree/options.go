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
	NoCache    bool
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

// bind defines the command-line flags on a flag set, and gives that set the
// help page as its usage, so -h and --help print the page rather than the
// defaults the flag package would list.
func (o *Options) bind(fs *flag.FlagSet) {
	fs.BoolVar(&o.Update, "u", false, "update the workspace dependencies that are behind their latest tag, and tidy")
	fs.BoolVar(&o.UpdateAll, "U", false, "update every dependency with go get -u ./..., including ones outside the workspace")
	fs.BoolVar(&o.Pull, "pull", false, "pull new changes for each git repository")
	fs.BoolVar(&o.All, "all", false, "include all modules (default: skip modules without releases/changes); with verdict, report every release in the chain")
	fs.BoolVar(&o.PUML, "puml", false, "output PlantUML dependency diagram to stdout")
	fs.BoolVar(&o.D2, "d2", false, "output D2 dependency diagram to stdout")
	fs.BoolVar(&o.Matrix, "t", false, "output dependency matrix to stdout")
	fs.BoolVar(&o.Verbose, "v", false, "verbose output: show module details, every untracked file, and commands run during updates")
	fs.BoolVar(&o.Verbose, "verbose", false, "alias of -v")
	fs.BoolVar(&o.Apply, "apply", false, "perform the resolution instead of rendering it")
	fs.BoolVar(&o.Stats, "stats", false, "collapse worktree verdict to a table of counts, one row per release")
	fs.BoolVar(&o.NoCache, "no-cache", false, "read every commit again instead of the models kept under the user cache directory")
	fs.StringVar(&o.From, "from", "", "the `REVISION` worktree verdict measures from, a tag by default; all, 0 or HEAD report every release")
	fs.StringVar(&o.To, "to", "", "the `REVISION` worktree verdict measures to, the working tree by default")
	fs.StringVar(&o.GoVersion, "go", "", "set the go directive of every go.mod and go.work to `VERSION`, then update dependencies")

	fs.Usage = func() { _ = writeHelp(os.Stdout, helpSpec(fs)) }
}

// helpSpec is the page the command prints.
func helpSpec(fs *flag.FlagSet) spec {
	return spec{
		Name:    "worktree",
		Tagline: "the state of a git and go workspace, and the releases it has earned",
		Usage: []string{
			"worktree [flags] [path]",
			"worktree <command> [flags] [path]",
		},
		Description: `With no command, worktree scans the workspace and writes what it found: one
row per module or repository, the tag it carries, the commits since that tag,
what it requires from the workspace and what requires it. The scan starts at
the nearest current or parent directory holding a go.work, a go.mod or a .git,
and honours the .gitignore files it meets on the way down.

The path is optional and selects the modules to work on. It matches a short
module name, a directory, or any part of either, so "worktree platform" and
"worktree ./platform" pick the same module. "./..." is the whole workspace,
which is what a run with no path reads anyway.`,
		Commands: []command{
			{commandConfig, "open the setup screen, which writes the configuration file"},
			{commandResolve, "release the selected modules in dependency order"},
			{commandVerdict, "report the release one module has earned, as markdown"},
			{releasePatch, "print the git commands that cut a patch release of the repository here"},
			{releaseMinor, "print the git commands that cut a minor release of the repository here"},
		},
		Flags: fs,
		Examples: []example{
			{"worktree", "the whole workspace, one row per module"},
			{"worktree platform -v", "one module, with its commits, issues and untracked files"},
			{"worktree -u", "update the workspace dependencies that moved, and tidy"},
			{"worktree resolve --apply", "release the modules that earned one, in dependency order"},
			{"worktree verdict --from all", "every release of the repository here, as markdown"},
			{"worktree patch | sh -x", "tag and push the next patch release"},
		},
		Notes: `patch, minor and verdict read the git repository of the current directory,
or the one the path names. Everything else reads the workspace around it.

resolve renders the plan and runs nothing until --apply is given.`,
	}
}

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
	opts.bind(flag.CommandLine)
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
