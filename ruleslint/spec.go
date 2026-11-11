package main

import "flag"

// helpSpec is the page the command prints.
func helpSpec(fs *flag.FlagSet) spec {
	return spec{
		Name:    "ruleslint",
		Tagline: "check the directories of a site against a rule file",
		Usage:   []string{"ruleslint [-v] [rulefile]"},
		Description: `Ruleslint reads a rule file, finds the directories its input names, and
checks every one of them against the rules.

A rule is "<kind> <path>", where the path is a glob inside the directory and
"a or b" accepts either. "require" is a path that has to exist, "pass" is one
that may, and counts when it does, and "error" is one that must not.`,
		Flags: fs,
		Examples: []example{
			{"ruleslint", "check with rules/module.yml of the current directory"},
			{"ruleslint -v", "also list every check of every module: ok, miss (a pass path absent) or FAIL"},
			{"ruleslint rules/module.yml", "check with a named rule file"},
		},
		Notes: `The input globs are resolved from the parent of the rule file's directory.

Per module the output is one line per failing require and per present error,
"<name> <dir>: <message>", then "<dir>: required met/total, passing n/m,
errors: n". A last line, "total: required met/total, passing n, errors: n",
sums the modules. With -v each module's checks come first, indented, one per
line.

Exits 1 when a rule fails, and 2 when the rule file cannot be read.`,
	}
}
