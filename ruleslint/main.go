// Command ruleslint checks the directories of a site against a rule file.
//
// The rule file names the directories to check, and per directory a list of
// rules: a path that has to exist, one that may, and one that must not.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	code, err := run(os.Args[1:], os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ruleslint:", err)
		os.Exit(2)
	}
	os.Exit(code)
}

func run(args []string, stdout, stderr io.Writer) (int, error) {
	fs := flag.NewFlagSet("ruleslint", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	verbose := fs.Bool("v", false, "list every check of every module, passed or not")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, writeHelp(stdout, helpSpec(fs))
		}
		return 0, err
	}
	if fs.NArg() > 1 {
		return 0, errors.New("takes one rule file at most")
	}

	file := "rules/module.yml"
	if fs.NArg() == 1 {
		file = fs.Arg(0)
	}

	set, err := load(file, stderr)
	var re *RuleErrors
	if errors.As(err, &re) {
		// report every bad rule, then fail
		for _, e := range re.Errs {
			fmt.Fprintln(stdout, e)
		}
		return 1, nil
	}
	if err != nil {
		return 0, err
	}

	res, err := set.check()
	if err != nil {
		return 0, err
	}
	res.write(stdout, set.Name, *verbose)

	if res.failed() {
		return 1, nil
	}
	return 0, nil
}
