// Package model holds a rule file as a tree, and the checks a run of it emits.
package model

// Config is a parsed rule file: the name of the structure, the directories it
// describes and the commands that lint them.
type Config struct {
	Name    string
	Dirs    []*Dir
	Linters []Linter
}

// Dir names the paths that exist under a "dir <pattern>" block. The root block
// of a file writes no pattern.
type Dir struct {
	Pattern string
	Line    int
	Entries []*Entry
}

// Entry holds one path pattern of a dir block and the rules written under it.
type Entry struct {
	Pattern string
	Line    int
	Rules   []Rule
}

// Rule names the text every file an entry matches holds, or holds nowhere when
// Deny is set.
type Rule struct {
	Text string
	Line int
	Deny bool
}

// Key is the rule as the file writes it: require or deny.
func (r Rule) Key() string {
	if r.Deny {
		return "deny"
	}
	return "require"
}

// Linter holds one command line of the linters block.
type Linter struct {
	Command string
	Line    int
}

// Check reports one rule applied to one path, and what the path failed.
type Check struct {
	Line   int    // the rule file line the rule is written on
	Path   string // the path checked, relative to the tree root
	Rule   string // the rule as the file writes it
	Detail string // what failed, empty when the check passed
	OK     bool
}

// Checks holds what a run emitted, in the order the rules are written.
type Checks []Check

// Failed counts the checks that did not pass.
func (c Checks) Failed() int {
	n := 0
	for _, one := range c {
		if !one.OK {
			n++
		}
	}
	return n
}
