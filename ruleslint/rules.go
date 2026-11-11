package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Kinds a rule takes.
const (
	kindRequire = "require" // a path has to exist
	kindPass    = "pass"    // a path may exist, and counts when it does
	kindError   = "error"   // a path must not exist
)

// Set is a rule file: what it checks, where, and by which rules.
type Set struct {
	Name   string            `yaml:"name"`
	Schema map[string]string `yaml:"schema"`
	Input  []struct {
		Dirs string `yaml:"dirs"`
	} `yaml:"input"`
	Rules []Rule `yaml:"rules"`

	// Root is where the input globs are resolved from: the directory that
	// holds the rules/ folder of the file.
	Root string `yaml:"-"`
}

// Rule is one line of "<kind> <path> [or <path>...]" and the message that
// says why it exists.
type Rule struct {
	Rule    string `yaml:"rule"`
	Message string `yaml:"message"`

	Kind  string   `yaml:"-"`
	Paths []string `yaml:"-"`
}

// load reads a rule file. A list item whose keys are indented unevenly, as
// "  - rule:" followed by a "message:" one column off, is a common slip; it is
// realigned with a warning rather than refused.
func load(file string, warn io.Writer) (*Set, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	set := &Set{}
	if err := yaml.Unmarshal(data, set); err != nil {
		fixed := realign(string(data))
		if fixed == string(data) || yaml.Unmarshal([]byte(fixed), set) != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		fmt.Fprintf(warn, "ruleslint: %s: %v; keys of a list item realigned\n", file, err)
	}

	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, err
	}
	set.Root = filepath.Dir(filepath.Dir(abs))

	// Every bad rule is reported, not the first.
	var errs []error
	for i := range set.Rules {
		r := &set.Rules[i]
		fields := strings.Fields(r.Rule)
		if len(fields) < 2 {
			errs = append(errs, fmt.Errorf("%s: rule %d %q: want \"<kind> <path>\"", file, i+1, r.Rule))
			continue
		}
		r.Kind = fields[0]
		switch r.Kind {
		case kindRequire, kindPass, kindError:
		default:
			errs = append(errs, fmt.Errorf("%s: rule %d %q: unknown kind %q", file, i+1, r.Rule, r.Kind))
			continue
		}
		r.Paths = strings.Split(strings.Join(fields[1:], " "), " or ")
		for j := range r.Paths {
			r.Paths[j] = strings.TrimSpace(r.Paths[j])
		}
	}
	if len(errs) > 0 {
		return nil, &RuleErrors{errs}
	}
	return set, nil
}

// RuleErrors is every bad rule of a file, so all of them are reported.
type RuleErrors struct{ Errs []error }

func (e *RuleErrors) Error() string { return errors.Join(e.Errs...).Error() }

var keyLine = regexp.MustCompile(`^(\s*)(- )?([A-Za-z_]+):`)

// realign moves the keys that follow the first key of a list item to its column.
func realign(src string) string {
	var out []string
	keyCol, dashCol := -1, -1
	for _, line := range strings.Split(src, "\n") {
		m := keyLine.FindStringSubmatch(line)
		switch {
		case m == nil:
		case m[2] != "":
			dashCol = len(m[1])
			keyCol = dashCol + 2
		case keyCol >= 0 && len(m[1]) > dashCol && len(m[1]) != keyCol:
			line = strings.Repeat(" ", keyCol) + strings.TrimLeft(line, " ")
		case len(m[1]) <= dashCol:
			keyCol, dashCol = -1, -1
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// Check is one rule applied to one directory.
type Check struct {
	Rule    string
	Kind    string
	Found   bool
	Message string
	Looked  []string // the paths tried, joined to the directory
	Hit     string   // the path that matched, when Found
}

// reason says what is wrong, in terms of the paths: what was required and
// where it was looked for, or what is present and must not be.
func (c Check) reason() string {
	if c.Kind == kindError {
		return "forbidden " + c.Rule + " is present at " + c.Hit
	}
	return "missing required " + c.Rule + ", looked for " + strings.Join(c.Looked, " or ")
}

// bad reports whether the check raised an error line: a require that is
// missing, or an error path that is present.
func (c Check) bad() bool {
	return (c.Kind == kindRequire && !c.Found) || (c.Kind == kindError && c.Found)
}

// ModuleResult is what one directory scored.
type ModuleResult struct {
	Name                          string
	Required, Met, Passing, Fails int
	Passes                        int // pass rules in the set
	Checks                        []Check
}

// Result is what a run found, per directory.
type Result struct {
	Errs                          []error // input patterns that could not be scanned
	Modules                       []*ModuleResult
	Required, Met, Passing, Fails int
}

func (r *Result) failed() bool {
	return len(r.Errs) > 0 || r.Met < r.Required || r.Fails > 0
}

// label names a directory: relative to the rule root when inside it, else its path.
func (s *Set) label(dir string) string {
	if rel, err := filepath.Rel(s.Root, dir); err == nil && len(rel) < len(dir) {
		return rel
	}
	return dir
}

// write prints, per module, a line per failure ("<name> <dir>: <message>"),
// then the module's summary. With verbose it also lists every check first.
// A last "total" line sums the modules.
func (r *Result) write(w io.Writer, name string, verbose bool) {
	label := strings.ToLower(name)
	for _, e := range r.Errs {
		fmt.Fprintf(w, "%s: %v\n", label, e)
	}
	for _, m := range r.Modules {
		if verbose {
			for _, c := range m.Checks {
				fmt.Fprintf(w, "  %-4s %s %s\n", c.status(), c.Kind, c.Rule)
			}
		}
		for _, c := range m.Checks {
			if c.bad() {
				fmt.Fprintf(w, "%s %s: %s: %s\n", label, m.Name, c.reason(), c.Message)
			}
		}
		fmt.Fprintf(w, "%s: required %d/%d, passing %d/%d, errors: %d\n",
			m.Name, m.Met, m.Required, m.Passing, m.Passes, m.Fails)
	}
	fmt.Fprintf(w, "total: required %d/%d, passing %d, errors: %d\n", r.Met, r.Required, r.Passing, r.Fails)
}

// status is the word a verbose line leads with.
func (c Check) status() string {
	switch {
	case c.bad():
		return "FAIL"
	case c.Kind == kindPass && !c.Found:
		return "miss"
	default:
		return "ok"
	}
}

// check runs every rule over every directory the input names.
func (s *Set) check() (*Result, error) {
	var dirs []string
	var bad []error
	for _, in := range s.Input {
		pat := in.Dirs
		if !filepath.IsAbs(pat) {
			pat = filepath.Join(s.Root, pat)
		}
		hits, err := filepath.Glob(pat)
		if err != nil {
			// a bad pattern is reported, the other inputs still scan
			bad = append(bad, fmt.Errorf("input %q: %w", in.Dirs, err))
			continue
		}
		for _, h := range hits {
			if fi, err := os.Stat(h); err == nil && fi.IsDir() {
				dirs = append(dirs, h)
			}
		}
	}
	sort.Strings(dirs)

	res := &Result{Errs: bad}
	for _, dir := range dirs {
		m := &ModuleResult{Name: s.label(dir)}
		for _, r := range s.Rules {
			found := false
			var looked []string
			hitPath := ""
			for _, p := range r.Paths {
				looked = append(looked, filepath.Join(dir, p))
				if hit, _ := filepath.Glob(filepath.Join(dir, p)); len(hit) > 0 && !found {
					found = true
					hitPath = hit[0]
				}
			}
			c := Check{Rule: strings.Join(r.Paths, " or "), Kind: r.Kind, Found: found, Message: r.Message, Looked: looked, Hit: hitPath}
			m.Checks = append(m.Checks, c)
			switch r.Kind {
			case kindRequire:
				m.Required++
				if found {
					m.Met++
				}
			case kindPass:
				m.Passes++
				if found {
					m.Passing++
				}
			}
			if c.bad() {
				m.Fails++
			}
		}
		res.Modules = append(res.Modules, m)
		res.Required += m.Required
		res.Met += m.Met
		res.Passing += m.Passing
		res.Fails += m.Fails
	}
	return res, nil
}
