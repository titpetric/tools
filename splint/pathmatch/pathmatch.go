// Package pathmatch matches a file path against a glob from a configuration
// file.
//
// path.Match is the syntax, and a star in it stops at a separator: parser/*.go
// is the files of one directory and says nothing about the directories under
// it. A configuration file names a kind of file, and the kind a tree names
// lives at every depth in the common case, so a pattern here reads ** as any
// number of segments. .gitignore, the shells and every CI configuration spell
// the same idea that way, so an author writes it with no prompting.
package pathmatch

import (
	"path"
	"strings"
)

// doubleStar is the segment that stands for any number of segments.
const doubleStar = "**"

// Match reports whether pattern covers name.
//
// Match reads the pattern in segments and matches every segment but ** with
// path.Match, so what one segment accepts is exactly what path.Match accepts:
// a star for any run of characters, a question mark for one, a character class
// in brackets. A ** segment matches any number of segments including none,
// so **/*_test.go reaches a test file at any depth and one in the root as
// well.
//
// A malformed pattern returns path.ErrBadPattern, and Match reads the whole
// pattern before it matches anything, so the error does not depend on the
// name it gets.
func Match(pattern, name string) (bool, error) {
	if err := Valid(pattern); err != nil {
		return false, err
	}
	return match(strings.Split(pattern, "/"), strings.Split(name, "/")), nil
}

// Valid reports a pattern Match can read. A loader runs this check, so the
// loader refuses a typo in a configuration file at its source, and the typo
// never matches nothing for the life of the file.
func Valid(pattern string) error {
	for _, segment := range strings.Split(pattern, "/") {
		if segment == doubleStar {
			continue
		}
		if _, err := path.Match(segment, ""); err != nil {
			return err
		}
	}
	return nil
}

// match walks the two segment lists. Valid has already read the pattern, so
// nothing here holds a malformed pattern and path.Match cannot fail.
func match(pattern, name []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == doubleStar {
			// ** stands for any number of segments, so every tail of the name
			// goes to the rest of the pattern. The shortest tail goes first,
			// and in that one ** stands for nothing.
			rest := pattern[1:]
			if len(rest) == 0 {
				return true
			}
			for index := 0; index <= len(name); index++ {
				if match(rest, name[index:]) {
					return true
				}
			}
			return false
		}

		if len(name) == 0 {
			return false
		}

		matched, _ := path.Match(pattern[0], name[0])
		if !matched {
			return false
		}

		pattern, name = pattern[1:], name[1:]
	}

	return len(name) == 0
}
