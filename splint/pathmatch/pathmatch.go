// Package pathmatch matches a file path against a glob written in a config
// file.
//
// path.Match is the syntax, and a star in it stops at a separator: parser/*.go
// is the files of one directory and says nothing about the directories under
// it. A config file names a kind of file rather than a directory listing, and
// the kind a tree wants to name is usually at every depth, so a pattern here
// reads ** as any number of segments. That is the spelling .gitignore, the
// shells and every CI config use for the same idea, so it is the one an author
// writes without being told.
package pathmatch

import (
	"path"
	"strings"
)

// doubleStar is the segment that stands for any number of segments.
const doubleStar = "**"

// Match reports whether name is covered by pattern.
//
// The pattern is read in segments and every segment but ** is matched with
// path.Match, so what one segment accepts is exactly what path.Match accepts:
// a star for any run of characters, a question mark for one, a character class
// in brackets. A ** segment matches any number of segments including none,
// which is what makes **/*_test.go reach a test file at any depth and one in
// the root as well.
//
// A malformed pattern returns path.ErrBadPattern, and the whole pattern is
// read before anything is matched, so the error does not depend on which name
// it was asked about.
func Match(pattern, name string) (bool, error) {
	if err := Valid(pattern); err != nil {
		return false, err
	}
	return match(strings.Split(pattern, "/"), strings.Split(name, "/")), nil
}

// Valid reports a pattern Match can read. It is the check a loader runs, so a
// typo in a config file is refused where it was written rather than left to
// match nothing for the life of the file.
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
// nothing here can be malformed and path.Match cannot fail.
func match(pattern, name []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == doubleStar {
			// ** stands for any number of segments, so every tail of the name
			// is offered to the rest of the pattern. The shortest tail is
			// tried first, which is the one where ** stands for nothing.
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
