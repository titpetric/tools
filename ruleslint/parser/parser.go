// Package parser reads a rule file into a model.Config.
//
// The file is line based: a line ending in an unquoted "{" opens a block, a
// line holding "}" closes one, and every other line is a key, a path or a
// statement of the block it stands in.
package parser

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/titpetric/tools/ruleslint/model"
)

// The blocks a rule file opens, named as a message to the reader names them.
const (
	kindRoot    = "ruleslint"
	kindDir     = "dir"
	kindEntry   = "entry"
	kindLinters = "linters"
	kindSkip    = "unknown"
)

// The key of the root block, and the statements an entry block takes.
const (
	keyName    = "name"
	keyRequire = "require"
	keyDeny    = "deny"
)

// The marks a path takes: may be absent, and stands for the paths none names.
const (
	markOptional = "?"
	markRest     = "..."
)

// ParseFile reads the rule file at path.
func ParseFile(path string) (*model.Config, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(path, src)
}

// Parse reads src as the rule file named name.
//
// Every fault of the file is reported, not the first: the error joins one
// "name:line: message" per fault.
func Parse(name string, src []byte) (*model.Config, error) {
	p := &parser{name: name, cfg: &model.Config{}}
	for i, raw := range strings.Split(string(src), "\n") {
		if line := strings.TrimSpace(uncomment(raw)); line != "" {
			p.read(i+1, line)
		}
	}
	for _, open := range p.stack {
		p.errorf(open.line, "%s block is never closed", open.kind)
	}
	if len(p.errs) > 0 {
		return nil, errors.Join(p.errs...)
	}
	return p.cfg, nil
}

// parser is one pass over a file: the config it fills, and the faults it kept.
type parser struct {
	name  string
	cfg   *model.Config
	stack []*block
	errs  []error
}

// block is an open brace: its kind, and the node under it.
type block struct {
	kind  string
	line  int
	dir   *model.Dir
	entry *model.Entry
}

// read takes one line of the file apart.
func (p *parser) read(n int, line string) {
	switch head, ok := opens(line); {
	case line == "}":
		p.pop(n)
	case ok:
		p.push(n, head)
	default:
		p.add(n, line)
	}
}

// push opens the block a header names, and a skipped block for one it refuses.
func (p *parser) push(n int, head string) {
	top := p.top()
	switch {
	case top == nil:
		if head != kindRoot {
			p.errorf(n, "want %q, got %q", "ruleslint {", head)
		}
		p.stack = append(p.stack, &block{kind: kindRoot, line: n})
	case top.kind == kindRoot:
		p.pushTop(n, head)
	case top.kind == kindDir:
		entry := newEntry(n, head)
		top.dir.Entries = append(top.dir.Entries, entry)
		p.stack = append(p.stack, &block{kind: kindEntry, line: n, entry: entry})
	default:
		p.errorf(n, "a %s block takes no block, got %q", top.kind, head)
		p.stack = append(p.stack, &block{kind: kindSkip, line: n})
	}
}

// pushTop opens a block of the ruleslint block: a directory or the linters.
func (p *parser) pushTop(n int, head string) {
	switch {
	case head == kindLinters:
		p.stack = append(p.stack, &block{kind: kindLinters, line: n})
	case head == kindDir || strings.HasPrefix(head, kindDir+" "):
		dir := newDir(n, strings.TrimSpace(strings.TrimPrefix(head, kindDir)))
		p.cfg.Dirs = append(p.cfg.Dirs, dir)
		p.stack = append(p.stack, &block{kind: kindDir, line: n, dir: dir})
	default:
		p.errorf(n, "unknown block %q, want %q or %q", head, "dir", "linters")
		p.stack = append(p.stack, &block{kind: kindSkip, line: n})
	}
}

// pop closes the innermost block.
func (p *parser) pop(n int) {
	if len(p.stack) == 0 {
		p.errorf(n, "%q closes no block", "}")
		return
	}
	p.stack = p.stack[:len(p.stack)-1]
}

// add stores a line that opens and closes no block, under the block it stands in.
func (p *parser) add(n int, line string) {
	top := p.top()
	switch {
	case top == nil:
		p.errorf(n, "%q stands outside a block", line)
	case top.kind == kindSkip:
	case top.kind == kindRoot:
		p.key(n, line)
	case top.kind == kindDir:
		p.path(n, top.dir, line)
	case top.kind == kindEntry:
		p.rule(n, top.entry, line)
	case top.kind == kindLinters:
		p.cfg.Linters = append(p.cfg.Linters, model.Linter{Command: line, Line: n})
	}
}

// key stores the "name: <value>" line of the ruleslint block.
func (p *parser) key(n int, line string) {
	switch key, value, ok := directive(line); {
	case !ok:
		p.errorf(n, "want a block or a key, got %q", line)
	case key != keyName:
		p.errorf(n, "unknown key %q in a %s block", key, kindRoot)
	default:
		p.cfg.Name = value
	}
}

// path stores one path of a dir block, and "..." as the paths none names.
func (p *parser) path(n int, dir *model.Dir, line string) {
	switch stmt, _, ok := statement(line); {
	case line == markRest:
		dir.Rest = true
	case ok && (stmt == keyRequire || stmt == keyDeny):
		p.errorf(n, "%q stands in a block under a path, not in a %s block", stmt, kindDir)
	default:
		dir.Entries = append(dir.Entries, newEntry(n, line))
	}
}

// rule stores one statement of an entry block: require or deny, and its text.
func (p *parser) rule(n int, entry *model.Entry, line string) {
	if stmt, text, ok := statement(line); ok && (stmt == keyRequire || stmt == keyDeny) {
		entry.Rules = append(entry.Rules, model.Rule{Text: text, Line: n, Deny: stmt == keyDeny})
		return
	}
	if key, _, ok := directive(line); ok && (key == keyRequire || key == keyDeny) {
		p.errorf(n, "%q takes no colon, write %q", key+":", key+" '<text>'")
		return
	}
	p.errorf(n, "an entry block takes %q and %q only, got %q", "require '<text>'", "deny '<text>'", line)
}

// newDir reads a dir header, where "?" prefixes a directory that may be absent.
func newDir(n int, pattern string) *model.Dir {
	trimmed, optional := strings.CutPrefix(pattern, markOptional)
	return &model.Dir{Pattern: trimmed, Line: n, Optional: optional}
}

// newEntry reads a path line, where "?" prefixes a path that may be absent.
func newEntry(n int, line string) *model.Entry {
	pattern, optional := strings.CutPrefix(line, markOptional)
	return &model.Entry{Pattern: pattern, Line: n, Optional: optional}
}

// top returns the innermost open block, and nil before the first brace.
func (p *parser) top() *block {
	if len(p.stack) == 0 {
		return nil
	}
	return p.stack[len(p.stack)-1]
}

// errorf keeps a fault of one line, in the form an editor jumps to.
func (p *parser) errorf(n int, format string, args ...any) {
	p.errs = append(p.errs, fmt.Errorf("%s:%d: %s", p.name, n, fmt.Sprintf(format, args...)))
}

// opens returns the header of a line that ends in an unquoted "{".
func opens(line string) (string, bool) {
	if !strings.HasSuffix(line, "{") {
		return "", false
	}
	var quote rune
	for i, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == '{' && i == len(line)-1:
			return strings.TrimSpace(line[:i]), true
		}
	}
	return "", false
}

// uncomment drops a "#" comment that opens outside quotes.
func uncomment(line string) string {
	var quote rune
	for i, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == '#':
			return line[:i]
		}
	}
	return line
}

var bareWord = regexp.MustCompile(`^[a-z][a-z_]*$`)

// directive splits "key: value", and unquotes a value written in quotes.
func directive(line string) (string, string, bool) {
	i := strings.IndexByte(line, ':')
	if i < 0 || !bareWord.MatchString(line[:i]) {
		return "", "", false
	}
	rest := line[i+1:]
	if rest != "" && !strings.HasPrefix(rest, " ") && !strings.HasPrefix(rest, "\t") {
		return "", "", false
	}
	value, _ := unquote(strings.TrimSpace(rest))
	return line[:i], value, true
}

// statement splits "<name> '<text>'", and refuses text written unquoted.
func statement(line string) (string, string, bool) {
	i := strings.IndexAny(line, " \t")
	if i < 0 || !bareWord.MatchString(line[:i]) {
		return "", "", false
	}
	text, ok := unquote(strings.TrimSpace(line[i+1:]))
	if !ok {
		return "", "", false
	}
	return line[:i], text, true
}

// unquote drops a pair of matching quotes, and reports whether it found one.
func unquote(value string) (string, bool) {
	if len(value) < 2 {
		return value, false
	}
	quote := value[0]
	if (quote == '\'' || quote == '"') && value[len(value)-1] == quote {
		return value[1 : len(value)-1], true
	}
	return value, false
}
