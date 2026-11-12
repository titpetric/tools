package main

import (
	"fmt"
	"io"

	"github.com/titpetric/tools/ruleslint/model"
)

// report writes a line per failing check, then the counts of the run.
func report(w io.Writer, file, name string, checks model.Checks) {
	for _, one := range checks {
		if !one.OK {
			fmt.Fprintln(w, line(one, file, one.Detail))
		}
	}
	if name == "" {
		name = "ruleslint"
	}
	fmt.Fprintf(w, "%s: %d checks, %d failed\n", name, len(checks), checks.Failed())
}

// line writes one check as "<file>:<line>: <path>: <text>", for an editor to jump by.
func line(one model.Check, file, text string) string {
	if one.Path == "" {
		return fmt.Sprintf("%s:%d: %s", file, one.Line, text)
	}
	return fmt.Sprintf("%s:%d: %s: %s", file, one.Line, one.Path, text)
}
