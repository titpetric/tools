package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// The palette a help page is painted in, which is the one the reports beside
// it are painted in: a heading is bold amber, a name is teal, and what a name
// defaults to is grey.
const (
	helpReset   = "\033[0m"
	helpSection = "\033[1;38;5;220m"
	helpName    = "\033[38;5;72m"
	helpDim     = "\033[38;5;245m"
)

// spec is a help page: what the tool is, how it is called, what it takes, and
// what a run of it looks like.
//
// A section with nothing in it is left out, so a tool with no subcommands has
// no Commands heading rather than an empty one.
type spec struct {
	// Name is the command as it is typed, and Tagline is the one line that
	// says what it is for.
	Name    string
	Tagline string

	// Usage are the forms the command takes, one per line, and Description is
	// the paragraphs under them.
	Usage       []string
	Description string

	// Commands are the subcommands, in the order a reader meets them.
	Commands []command

	// Flags is the parser itself, walked rather than repeated: a flag is
	// documented by having been defined.
	Flags *flag.FlagSet

	// Examples are runs worth copying, and Notes is what is left to say.
	Examples []example
	Notes    string
}

// command is one subcommand and what it does.
type command struct {
	Name  string
	About string
}

// example is one run and what it is for.
type example struct {
	Command string
	About   string
}

// flagDoc is one flag as a page reads it: the name, what its value is called,
// what it defaults to, and what it does.
type flagDoc struct {
	Name        string
	Placeholder string
	Default     string
	About       string
}

// writeHelp writes the page the way its reader reads it: painted for a
// terminal, and markdown for a file, a pipe or a program.
func writeHelp(w io.Writer, s spec) error {
	if isTerminal(w) {
		return helpTerminal(w, s)
	}
	return helpMarkdown(w, s)
}

// helpTerminal writes the page a person reads.
func helpTerminal(w io.Writer, s spec) error {
	out := &strings.Builder{}

	fmt.Fprintf(out, "%s%s%s - %s\n", helpSection, s.Name, helpReset, s.Tagline)

	if len(s.Usage) > 0 {
		fmt.Fprintf(out, "\n%sUsage:%s\n", helpSection, helpReset)
		for _, line := range s.Usage {
			fmt.Fprintf(out, "  %s\n", line)
		}
	}

	if s.Description != "" {
		fmt.Fprintf(out, "\n%s\n", strings.TrimSpace(s.Description))
	}

	if len(s.Commands) > 0 {
		fmt.Fprintf(out, "\n%sCommands:%s\n", helpSection, helpReset)
		width := 0
		for _, one := range s.Commands {
			width = max(width, len(one.Name))
		}
		for _, one := range s.Commands {
			fmt.Fprintf(out, "  %s%s%s%s  %s\n", helpName, one.Name, helpReset,
				strings.Repeat(" ", width-len(one.Name)), one.About)
		}
	}

	if docs := flagDocs(s.Flags); len(docs) > 0 {
		fmt.Fprintf(out, "\n%sFlags:%s\n", helpSection, helpReset)
		width := 0
		for _, one := range docs {
			width = max(width, len(one.spelled()))
		}
		for _, one := range docs {
			fmt.Fprintf(out, "  %s%s%s%s  %s", helpName, one.spelled(), helpReset,
				strings.Repeat(" ", width-len(one.spelled())), one.About)
			if one.Default != "" {
				fmt.Fprintf(out, " %s(default %s)%s", helpDim, one.Default, helpReset)
			}
			fmt.Fprintln(out)
		}
	}

	if len(s.Examples) > 0 {
		fmt.Fprintf(out, "\n%sExamples:%s\n", helpSection, helpReset)
		for _, one := range s.Examples {
			fmt.Fprintf(out, "  %s%s%s\n      %s\n", helpName, one.Command, helpReset, one.About)
		}
	}

	if s.Notes != "" {
		fmt.Fprintf(out, "\n%s\n", strings.TrimSpace(s.Notes))
	}

	_, err := io.WriteString(w, out.String())
	return err
}

// helpMarkdown writes the page a document holds and a program reads.
func helpMarkdown(w io.Writer, s spec) error {
	out := &strings.Builder{}

	fmt.Fprintf(out, "# %s\n\n%s\n", s.Name, s.Tagline)

	if len(s.Usage) > 0 {
		fmt.Fprintf(out, "\n## Usage\n\n```\n%s\n```\n", strings.Join(s.Usage, "\n"))
	}

	if s.Description != "" {
		fmt.Fprintf(out, "\n%s\n", strings.TrimSpace(s.Description))
	}

	if len(s.Commands) > 0 {
		fmt.Fprint(out, "\n## Commands\n\n| Command | What it does |\n|---|---|\n")
		for _, one := range s.Commands {
			fmt.Fprintf(out, "| `%s` | %s |\n", one.Name, one.About)
		}
	}

	if docs := flagDocs(s.Flags); len(docs) > 0 {
		fmt.Fprint(out, "\n## Flags\n\n| Flag | Default | What it does |\n|---|---|---|\n")
		for _, one := range docs {
			fmt.Fprintf(out, "| `%s` | %s | %s |\n", one.spelled(), code(one.Default), one.About)
		}
	}

	if len(s.Examples) > 0 {
		fmt.Fprint(out, "\n## Examples\n")
		for _, one := range s.Examples {
			fmt.Fprintf(out, "\n```\n%s\n```\n\n%s\n", one.Command, one.About)
		}
	}

	if s.Notes != "" {
		fmt.Fprintf(out, "\n%s\n", strings.TrimSpace(s.Notes))
	}

	_, err := io.WriteString(w, out.String())
	return err
}

// spelled is the flag as it is typed, with the name of its value after it
// where it takes one.
func (f flagDoc) spelled() string {
	if f.Placeholder == "" {
		return "-" + f.Name
	}
	return "-" + f.Name + " " + f.Placeholder
}

// flagDocs reads the flags off the parser, in the order it walks them, which
// is the order a reader looks them up in.
//
// A default that is the zero of its type says nothing, so it is left out: a
// bool that is off and a string that is empty are what a flag not given
// already means.
func flagDocs(fs *flag.FlagSet) []flagDoc {
	if fs == nil {
		return nil
	}

	var out []flagDoc
	fs.VisitAll(func(one *flag.Flag) {
		placeholder, about := flag.UnquoteUsage(one)

		value := one.DefValue
		switch value {
		case "", "false", "0":
			value = ""
		}

		out = append(out, flagDoc{
			Name:        one.Name,
			Placeholder: placeholder,
			Default:     value,
			About:       about,
		})
	})

	return out
}

// code writes a cell as code, and writes an empty cell as nothing rather than
// as an empty pair of backticks.
func code(value string) string {
	if value == "" {
		return ""
	}
	return "`" + value + "`"
}

// isTerminal reports whether a writer is a terminal, which is what decides
// whether the page carries colour. A dumb terminal is not one: it is what a
// pager or an editor sets when it wants plain text.
func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok || os.Getenv("TERM") == "dumb" {
		return false
	}

	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
