// Package test runs the Go tests a change reaches.
//
// A run compares the working tree with a revision, HEAD by default, and keeps
// the functions whose signature or body moved. A changed test runs itself, a
// changed benchmark runs under -benchtime 10s, and a changed function runs the
// tests that name it or reach it through the package's own helpers.
//
// A doc comment and a type declaration are not changes here: go build already
// judges a type, and a comment changes no behaviour. The command is quiet by
// default and writes one "PASS TestName" or "FAIL TestName" line per test, so
// a pipeline reads it without parsing anything else.
package test

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// Options is what one splint test run asks for.
type Options struct {
	// SourcePath is the tree to read, and Pattern says how far below it to
	// look: "." is the package there and "./..." everything under it.
	SourcePath string
	Pattern    string

	// Base is the revision the working tree is compared with.
	Base string

	// Split runs every test in an invocation of its own.
	Split bool

	// List writes what a run would start, and starts nothing.
	List bool

	// Verbose says what was read and which command ran.
	Verbose bool

	// GoFlags are the flags the command line did not recognise, forwarded to
	// every go test invocation.
	GoFlags []string
}

// valueFlags are the go test flags that take a value in the next argument.
// A flag not here is a boolean, and the argument after it is a pattern.
var valueFlags = map[string]bool{
	"bench": true, "benchtime": true, "blockprofile": true,
	"blockprofilerate": true, "count": true, "covermode": true,
	"coverpkg": true, "coverprofile": true, "cpu": true,
	"cpuprofile": true, "exec": true, "fuzz": true, "fuzzminimizetime": true,
	"fuzztime": true, "gcflags": true, "ldflags": true, "list": true,
	"memprofile": true, "memprofilerate": true, "mutexprofile": true,
	"mutexprofilefraction": true, "o": true, "outputdir": true,
	"p": true, "parallel": true, "run": true, "shuffle": true, "skip": true,
	"tags": true, "timeout": true, "trace": true, "vet": true,
}

// ParseOptions reads the command line of splint test.
//
// Every flag splint does not know is forwarded to go test, and a value flag
// takes the next argument only when go test documents a value.
func ParseOptions(args []string) (Options, bool, error) {
	opts := Options{SourcePath: ".", Pattern: ".", Base: "HEAD"}

	var help, patternSet bool
	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "-h" || arg == "--help":
			help = true

		case arg == "--split":
			opts.Split = true

		case arg == "--list":
			opts.List = true

		case arg == "-v" || arg == "--verbose":
			opts.Verbose = true

		case arg == "--":
			opts.GoFlags = append(opts.GoFlags, args[i+1:]...)
			i = len(args)

		case arg == "-i" || arg == "--i" || arg == "--source":
			i++
			if i >= len(args) {
				return opts, false, fmt.Errorf("%s takes a path", arg)
			}
			opts.SourcePath = args[i]

		case strings.HasPrefix(arg, "--i="):
			opts.SourcePath = strings.TrimPrefix(arg, "--i=")

		case strings.HasPrefix(arg, "-i="):
			opts.SourcePath = strings.TrimPrefix(arg, "-i=")

		case strings.HasPrefix(arg, "--source="):
			opts.SourcePath = strings.TrimPrefix(arg, "--source=")

		case arg == "--base" || arg == "--ref":
			i++
			if i >= len(args) {
				return opts, false, fmt.Errorf("%s takes a revision", arg)
			}
			opts.Base = args[i]

		case strings.HasPrefix(arg, "--base="):
			opts.Base = strings.TrimPrefix(arg, "--base=")

		case strings.HasPrefix(arg, "--ref="):
			opts.Base = strings.TrimPrefix(arg, "--ref=")

		case strings.HasPrefix(arg, "-") && arg != "-":
			opts.GoFlags = append(opts.GoFlags, arg)
			if strings.Contains(arg, "=") {
				continue
			}
			name := strings.TrimLeft(arg, "-")
			if name == "args" {
				opts.GoFlags = append(opts.GoFlags, args[i+1:]...)
				i = len(args)
				continue
			}
			if valueFlags[name] && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				opts.GoFlags = append(opts.GoFlags, args[i])
			}

		default:
			if patternSet {
				return opts, false, fmt.Errorf("one pattern, and %q is a second", arg)
			}
			opts.Pattern = arg
			patternSet = true
		}
	}

	return opts, help, nil
}

// Run starts the tests the change reaches and reports their verdicts.
//
// The exit code is 0 when every selected test passed, 1 when one failed, and
// an error is the run itself failing, which main reports as 2.
func Run(ctx context.Context, opts Options, stdout, stderr io.Writer) (int, error) {
	src, err := newSource(ctx, opts)
	if err != nil {
		return 0, err
	}

	changes, err := src.changes(ctx)
	if err != nil {
		return 0, err
	}

	changed, err := src.changedFuncs(ctx, changes)
	if err != nil {
		return 0, err
	}

	inv, err := src.inventory(ctx, opts.Pattern == "./...", changedDirs(changes, changed))
	if err != nil {
		return 0, err
	}

	if opts.Verbose {
		for _, one := range changed {
			fmt.Fprintf(stderr, "splint test: changed %s (%s)\n", one.Key, one.Dir)
		}
	}

	selected := selectChanged(inv, changed, opts.Verbose, stderr)

	if opts.List {
		return 0, selected.writeList(stdout)
	}

	return selected.run(ctx, src, opts, stdout, stderr)
}

// WriteHelp writes the page splint test answers a help request with.
func WriteHelp(w io.Writer) error {
	out := `# splint test

splint test [flags] [pattern] [-- go test flags]

Run the Go tests a change reaches. The run compares the working tree with
HEAD, or the revision --base names, and keeps the functions whose signature
or body moved.

A changed test runs itself. A changed benchmark runs under -benchtime 10s.
A changed function runs the tests that name it: Test<Func>, and for a method
on type T every Test<T>*. A test that reaches the function through a helper
in its own package runs too. Doc comments and type declarations are not
changes: go build judges a type, and a comment changes no behaviour.

Coverage is collected for every invocation and merged into a scan: one row
per changed symbol with its covered and measured lines, and the share of the
tree's tests that ran. Internal symbols are marked indirect; an internal
method is followed to the functions on its type that call it.

Flags splint does not know are forwarded to every go test invocation.

Flags:
  -i, --i PATH       read the tree at PATH (default ".")
      --base REV     compare the working tree with REV (default "HEAD")
      --split        run every test in an invocation of its own
      --list         write what would run, and run nothing
  -v, --verbose      say what was read and which command ran
  -h, --help         print this page

Exits 1 when a test failed, 2 when the run itself failed.
`
	_, err := io.WriteString(w, out)
	return err
}
