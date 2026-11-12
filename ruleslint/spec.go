package main

import (
	"flag"
)

// helpSpec is the page the command prints.
func helpSpec(fs *flag.FlagSet) spec {
	return spec{
		Name:    "ruleslint",
		Tagline: "check a source tree against the structure a rule file names",
		Usage:   []string{"ruleslint [rulefile]"},
		Description: `Ruleslint reads a rule file, resolves the directories it describes and
checks the paths of every one of them.

A rule file is a tree of blocks. "dir <pattern>" opens a directory, and every
bare line under it is a path that has to exist: a trailing slash wants a
directory, "*" stands for a run of characters, and "{name}" binds what it
matched for the rules under it. A path with a block under it takes
"require '<text>'", which every file it matched holds, and "deny '<text>'",
which none of them may. The commands of the "linters" block run in the root of
the tree.

A pattern holding a {name} nothing bound discovers what is there: it checks
the paths that match and wants none. A "?" before a path checks the paths that
match and wants none either, and before a dir pattern takes a directory that
is absent.

A "dir <pattern>" block names every path of the directory: one no line of the
block matches fails the run. A "..." line takes the rest of them, and the
"dir" block that names no pattern takes the rest of the tree.`,
		Flags: fs,
		Examples: []example{
			{"ruleslint", "check the tree against .ruleslint.conf of the current directory"},
			{"ruleslint site/php.conf", "check the site directory against a named rule file"},
			{"ruleslint 2>/dev/null", "drop what the linters write and keep the report"},
		},
		Notes: `The tree checked is the directory the rule file is in.

A failing check reads "<rulefile>:<line>: <path>: <what failed>", and the last
line counts the checks of the run. The report goes to stdout, and a linter
writes to stderr.

Exits 1 when a check failed, and 2 when the rule file does not parse.`,
	}
}
