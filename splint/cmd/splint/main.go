// Command splint lints a Go tree through the splint model.
//
// It parses the tree with one of two parsers, runs the linters over the
// document, and writes what they found: drawn for a terminal, and as a
// markdown table for anything else.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	code, err := run(ctx, os.Args[1:], os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "splint:", err)
		os.Exit(2)
	}
	os.Exit(code)
}
