// Command ruleslint checks a source tree against the structure a rule file names.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/titpetric/tools/ruleslint/checker"
	"github.com/titpetric/tools/ruleslint/parser"
)

func main() {
	code, err := run(os.Args[1:], os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ruleslint:", err)
		os.Exit(2)
	}
	os.Exit(code)
}

// run checks the tree and writes the report, answering 1 when a check failed
// and 2 when the rule file does not parse.
func run(args []string, stdout, stderr io.Writer) (int, error) {
	fs := flag.NewFlagSet("ruleslint", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, writeHelp(stdout, helpSpec(fs))
		}
		return 0, err
	}
	if fs.NArg() > 1 {
		return 0, errors.New("takes one rule file at most")
	}

	file := ".ruleslint.conf"
	if fs.NArg() == 1 {
		file = fs.Arg(0)
	}

	cfg, err := parser.ParseFile(file)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2, nil
	}

	checks, err := checker.Runner{Root: filepath.Dir(file), Out: stderr}.Run(cfg)
	if err != nil {
		return 0, err
	}

	report(stdout, file, cfg.Name, checks)
	if checks.Failed() > 0 {
		return 1, nil
	}
	return 0, nil
}
