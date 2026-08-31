package simpleparser

import (
	"strings"
)

// source is one file, split into lines and stripped of what a scanner must not
// read as code: the inside of a string, a rune or a comment.
//
// Stripping is done once, up front, and the two views are kept side by side.
// Every anchor the parser looks for is matched against the stripped line, so a
// brace in a string literal never closes a function and the word "func" in a
// comment never opens one. Everything the parser reports is taken from the
// line as it was written.
type source struct {
	// name is the base filename, which is what a declaration records.
	name string

	// lines is the file as it was written, one entry per line.
	lines []string

	// code is lines with the strings and comments blanked out, so a scan for
	// a brace or a keyword reads only code.
	code []string

	// comment reports which lines are wholly a comment, which is how a doc
	// comment is found above a declaration.
	comment []bool

	// commentAt is where the comment on a line begins, and is -1 for a line
	// carrying none. A backtick inside a comment is not a struct tag, and the
	// stripped view cannot say so: it blanks the comment and the tag alike.
	commentAt []int
}

// newSource reads a file into the two views.
func newSource(name string, data []byte) *source {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")

	src := &source{
		name:      name,
		lines:     lines,
		code:      make([]string, len(lines)),
		comment:   make([]bool, len(lines)),
		commentAt: make([]int, len(lines)),
	}

	inBlockComment := false
	for i, line := range lines {
		src.code[i], src.commentAt[i], inBlockComment = strip(line, inBlockComment)
		trimmed := strings.TrimSpace(line)
		src.comment[i] = strings.TrimSpace(src.code[i]) == "" &&
			(strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || inBlockComment || strings.HasSuffix(trimmed, "*/"))
	}

	return src
}

// len is how many lines the file has.
func (s *source) len() int {
	return len(s.lines)
}

// line returns one line as it was written, and an empty string past the end.
func (s *source) line(i int) string {
	if i < 0 || i >= len(s.lines) {
		return ""
	}
	return s.lines[i]
}

// codeLine returns one line with the strings and comments blanked out.
func (s *source) codeLine(i int) string {
	if i < 0 || i >= len(s.code) {
		return ""
	}
	return s.code[i]
}

// isComment reports whether a line is wholly a comment.
func (s *source) isComment(i int) bool {
	return i >= 0 && i < len(s.comment) && s.comment[i]
}

// code and comment split a line at the comment on it, which is where a struct
// tag stops being one.
func (s *source) split(i int) (code, comment string) {
	line := s.line(i)
	at := -1
	if i >= 0 && i < len(s.commentAt) {
		at = s.commentAt[i]
	}
	if at < 0 || at > len(line) {
		return line, ""
	}
	return line[:at], line[at:]
}

// text returns the lines from through to, as they were written, joined back
// into one string. It is what a declaration records as its source.
func (s *source) text(from, to int) string {
	if from < 0 {
		from = 0
	}
	if to >= len(s.lines) {
		to = len(s.lines) - 1
	}
	if from > to {
		return ""
	}
	return strings.Join(s.lines[from:to+1], "\n")
}

// strip blanks out everything on a line that is not code: the inside of a
// string, a raw string, a rune, and a comment. The blanks are spaces, so every
// column of the stripped line lines up with the column it came from.
//
// It returns the line, where the comment on it begins, and whether a block
// comment is still open at the end of it, which is what the next line has to
// be read under.
func strip(line string, inBlockComment bool) (string, int, bool) {
	out := []byte(line)
	comment := -1

	var (
		inString bool
		inRaw    bool
		inRune   bool
	)

	for i := 0; i < len(out); i++ {
		c := out[i]

		switch {
		case inBlockComment:
			if c == '*' && i+1 < len(out) && out[i+1] == '/' {
				out[i], out[i+1] = ' ', ' '
				i++
				inBlockComment = false
				continue
			}
			out[i] = ' '

		case inRaw:
			if c == '`' {
				inRaw = false
			}
			out[i] = ' '

		case inString:
			if c == '\\' && i+1 < len(out) {
				out[i], out[i+1] = ' ', ' '
				i++
				continue
			}
			if c == '"' {
				inString = false
			}
			out[i] = ' '

		case inRune:
			if c == '\\' && i+1 < len(out) {
				out[i], out[i+1] = ' ', ' '
				i++
				continue
			}
			if c == '\'' {
				inRune = false
			}
			out[i] = ' '

		default:
			switch {
			case c == '/' && i+1 < len(out) && out[i+1] == '/':
				// A line comment reaches the end of the line.
				comment = i
				for ; i < len(out); i++ {
					out[i] = ' '
				}
				return string(out), comment, false
			case c == '/' && i+1 < len(out) && out[i+1] == '*':
				if comment < 0 {
					comment = i
				}
				out[i], out[i+1] = ' ', ' '
				i++
				inBlockComment = true
			case c == '"':
				inString = true
				out[i] = ' '
			case c == '`':
				inRaw = true
				out[i] = ' '
			case c == '\'':
				inRune = true
				out[i] = ' '
			}
		}
	}

	// A string or a rune does not survive a line break in Go; a raw string and
	// a block comment do.
	return string(out), comment, inBlockComment
}
