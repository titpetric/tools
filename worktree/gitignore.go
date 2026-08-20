package main

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ignoreRule is a single parsed .gitignore pattern.
type ignoreRule struct {
	// segments is the pattern split on "/". Patterns without a slash are
	// normalized to a leading "**" so they match at any depth.
	segments []string
	negate   bool
	dirOnly  bool
}

// match reports whether rel, a slash separated path relative to the directory
// holding the .gitignore, matches the rule. isDir marks rel as a directory.
func (r ignoreRule) match(rel string, isDir bool) bool {
	if r.dirOnly && !isDir {
		return false
	}
	return matchSegments(r.segments, strings.Split(rel, "/"))
}

// matchSegments matches a pattern against a path, both split on "/". A "**"
// segment spans any number of path segments, other segments are matched with
// path.Match, which never matches across a separator.
func matchSegments(pattern, name []string) bool {
	if len(pattern) == 0 {
		return len(name) == 0
	}
	if pattern[0] == "**" {
		// A trailing "**" matches the contents of a directory, not the
		// directory itself.
		if len(pattern) == 1 {
			return len(name) > 0
		}
		for i := range len(name) + 1 {
			if matchSegments(pattern[1:], name[i:]) {
				return true
			}
		}
		return false
	}
	if len(name) == 0 {
		return false
	}
	if ok, err := path.Match(pattern[0], name[0]); err != nil || !ok {
		return false
	}
	return matchSegments(pattern[1:], name[1:])
}

// parseIgnoreRule parses one .gitignore line. It reports ok=false for blank
// lines and comments.
func parseIgnoreRule(line string) (ignoreRule, bool) {
	// Trailing whitespace is insignificant unless escaped with a backslash.
	for len(line) > 0 && (line[len(line)-1] == ' ' || line[len(line)-1] == '\t') {
		if strings.HasSuffix(line, `\ `) {
			break
		}
		line = line[:len(line)-1]
	}
	line = strings.ReplaceAll(line, `\ `, " ")

	if line == "" || strings.HasPrefix(line, "#") {
		return ignoreRule{}, false
	}

	var rule ignoreRule
	if strings.HasPrefix(line, "!") {
		rule.negate = true
		line = line[1:]
	} else if strings.HasPrefix(line, `\`) {
		// An escaped leading "#" or "!" is a literal character.
		line = line[1:]
	}
	if strings.HasSuffix(line, "/") {
		rule.dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}
	if line == "" {
		return ignoreRule{}, false
	}

	// A pattern with a slash anywhere but at the end is relative to the
	// .gitignore, any other pattern matches a name at any depth.
	anchored := strings.Contains(line, "/")
	line = strings.TrimPrefix(line, "/")
	rule.segments = strings.Split(line, "/")
	if !anchored {
		rule.segments = append([]string{"**"}, rule.segments...)
	}
	return rule, true
}

// ignoreFile holds the rules of one .gitignore, and the directory they are
// relative to.
type ignoreFile struct {
	dir   string // slash separated, as passed to the walk function
	rules []ignoreRule
}

// readIgnoreFile parses the .gitignore in dir. It returns nil when the
// directory has no readable .gitignore.
func readIgnoreFile(dir string) *ignoreFile {
	f, err := os.Open(filepath.Join(dir, ".gitignore"))
	if err != nil {
		return nil
	}
	defer f.Close()

	file := &ignoreFile{dir: filepath.ToSlash(dir)}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if rule, ok := parseIgnoreRule(scanner.Text()); ok {
			file.rules = append(file.rules, rule)
		}
	}
	if scanner.Err() != nil || len(file.rules) == 0 {
		return nil
	}
	return file
}

// ignoreStack is the set of .gitignore files that apply to the directory
// currently being walked, ordered from the outermost to the innermost.
type ignoreStack []*ignoreFile

// prune drops the .gitignore files of directories that the walked path is not
// below. The walk visits siblings after descending into a directory, so the
// stack has to be unwound before the rules are used again.
func (s ignoreStack) prune(walkPath string) ignoreStack {
	walkPath = filepath.ToSlash(walkPath)
	for len(s) > 0 && !isSlashSubpath(s[len(s)-1].dir, walkPath) {
		s = s[:len(s)-1]
	}
	return s
}

// push adds the .gitignore of dir to the stack, if it has one.
func (s ignoreStack) push(dir string) ignoreStack {
	if file := readIgnoreFile(dir); file != nil {
		s = append(s, file)
	}
	return s
}

// ignored reports whether the walked path is excluded by any .gitignore on the
// stack. The innermost file wins, and within a file the last matching pattern
// wins, as in git.
func (s ignoreStack) ignored(walkPath string, isDir bool) bool {
	walkPath = filepath.ToSlash(walkPath)
	for i := len(s) - 1; i >= 0; i-- {
		rel := slashRel(s[i].dir, walkPath)
		if rel == "" || rel == "." {
			continue
		}
		for j := len(s[i].rules) - 1; j >= 0; j-- {
			if s[i].rules[j].match(rel, isDir) {
				return !s[i].rules[j].negate
			}
		}
	}
	return false
}

// slashRel returns child relative to parent, or "" when child is not below it.
func slashRel(parent, child string) string {
	parent = strings.TrimSuffix(parent, "/")
	if parent == "." {
		return strings.TrimPrefix(child, "./")
	}
	if child == parent {
		return "."
	}
	if !strings.HasPrefix(child, parent+"/") {
		return ""
	}
	return child[len(parent)+1:]
}

// isSlashSubpath reports whether child is equal to or below parent.
func isSlashSubpath(parent, child string) bool {
	return slashRel(parent, child) != ""
}
