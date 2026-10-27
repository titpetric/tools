package simpleparser

import (
	"strings"
)

// source is one file, split into lines and stripped of what a scanner must not
// read as code: the inside of a string, a rune or a comment.
//
// The source strips once, up front, and keeps the two views side by side.
// The parser matches every anchor against the stripped line, so a
// brace in a string literal never closes a function and the word "func" in a
// comment never opens one. Everything the parser reports comes from the
// raw line.
type source struct {
	// name is the base filename, the form a declaration records.
	name string

	// lines is the raw file, one entry per line.
	lines []string

	// read is what the scan knows about each line, one entry per line.
	//
	// The four facts share one slice, which is one allocation
	// per file and one cache line: a scan
	// reads a line once, and reads every fact of it.
	read []lineInfo
}

// lineInfo is what stripping one line established about it.
type lineInfo struct {
	// code is the line with the strings and comments blanked out, so a scan
	// for a brace or a keyword reads only code.
	code string

	// commentAt is where the comment on the line begins, and is -1 for a line
	// carrying none. A backtick inside a comment is not a struct tag, and the
	// stripped view cannot say so: it blanks the comment and the tag alike.
	commentAt int32

	// comment reports a line that is wholly a comment, the test for a doc
	// comment above a declaration.
	comment bool

	// raw reports a line that left a raw string open, the test that tells a
	// declaration that has not ended from one that reads as though it has.
	raw bool
}

// newSource reads a file into the two views.
func newSource(name string, data []byte) *source {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")

	src := &source{
		name:  name,
		lines: lines,
		read:  make([]lineInfo, len(lines)),
	}

	var open openState
	for i, line := range lines {
		code, at, next := strip(line, open)

		src.read[i] = lineInfo{
			code:      code,
			commentAt: int32(at),
			raw:       next.raw,
			comment:   wholeComment(line, code, open, next),
		}
		open = next
	}

	return src
}

// wholeComment reports a line that is nothing but a comment, the test for a
// doc comment above a declaration.
func wholeComment(line, code string, before, after openState) bool {
	if strings.TrimSpace(code) != "" || after.raw {
		return false
	}

	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") ||
		before.comment || strings.HasSuffix(trimmed, "*/")
}

// len is how many lines the file has.
func (s *source) len() int {
	return len(s.lines)
}

// line returns one raw line, and an empty string past the end.
func (s *source) line(i int) string {
	if i < 0 || i >= len(s.lines) {
		return ""
	}
	return s.lines[i]
}

// codeLine returns one line with the strings and comments blanked out.
func (s *source) codeLine(i int) string {
	if i < 0 || i >= len(s.read) {
		return ""
	}
	return s.read[i].code
}

// rawOpen reports whether a line left a raw string open, which is a
// declaration that has not ended, whatever its code reads like.
func (s *source) rawOpen(i int) bool {
	return i >= 0 && i < len(s.read) && s.read[i].raw
}

// isComment reports whether a line is wholly a comment.
func (s *source) isComment(i int) bool {
	return i >= 0 && i < len(s.read) && s.read[i].comment
}

// code and comment split a line at the comment on it, the place where a struct
// tag stops being one.
func (s *source) split(i int) (code, comment string) {
	line := s.line(i)
	at := -1
	if i >= 0 && i < len(s.read) {
		at = int(s.read[i].commentAt)
	}
	if at < 0 || at > len(line) {
		return line, ""
	}
	return line[:at], line[at:]
}

// text returns the lines from through to, raw, joined back
// into one string.
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

// declText is text with the spacing before a comment trailing the last line
// rewritten the way go/printer writes it, which is a declaration's source as
// the ast collector records it. The printer separates the comment from the
// code with a tab where the source may hold a single space, so the raw line
// differs in whitespace alone.
func (s *source) declText(from, to int) string {
	if from < 0 {
		from = 0
	}
	if to >= len(s.lines) {
		to = len(s.lines) - 1
	}
	if from > to {
		return ""
	}

	last := s.lines[to]
	if to < len(s.read) && !s.read[to].raw {
		at := int(s.read[to].commentAt)
		if at >= 0 && at <= len(last) && strings.TrimSpace(s.read[to].code[at:]) == "" {
			comment := last[at:]
			last = strings.TrimRight(last[:at], " \t") + "\t" + comment
			if strings.HasPrefix(comment, "//") {
				// The printer ends a line comment with a newline, which the
				// node it was attached to then carries at its end.
				last += "\n"
			}
		}
	}

	if from == to {
		return last
	}
	lines := make([]string, 0, to-from+1)
	lines = append(lines, s.lines[from:to]...)
	return strings.Join(append(lines, last), "\n")
}

// strip blanks out everything on a line that is not code: the inside of a
// string, a raw string, a rune, and a comment. The blanks are spaces, so every
// column of the stripped line lines up with the column it came from.
//
// It returns the line, where the comment on it begins, and what is still open
// at the end of it, the state the scan carries into the next line.
func strip(line string, open openState) (string, int, openState) {
	// Most lines hold nothing to blank, and a line that holds
	// nothing is the line itself: looking is cheaper than blanking, and the
	// line goes on sharing the memory of the file it came from.
	if open == (openState{}) && !strings.ContainsAny(line, "\"`'/") {
		return line, -1, open
	}

	out := []byte(line)
	comment := -1

	var (
		inString bool
		inRune   bool
	)
	inBlockComment, inRaw := open.comment, open.raw

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
				return string(out), comment, openState{}
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
	// a block comment do, so the scan carries both to the next line.
	return string(out), comment, openState{comment: inBlockComment, raw: inRaw}
}

// openState is what a line left unfinished for the next one to read under.
type openState struct {
	// comment is a block comment the line opened and did not close.
	comment bool

	// raw is a raw string the line opened and did not close. A brace or a
	// keyword inside one is text, and a scan that read it as code would close
	// a declaration in the middle of a template.
	raw bool
}

// codeLines is how many lines of the file carry code, blanks and comments left
// out, the unit a measurement of a package counts.
//
// The count reads the raw line, and skips the stripped one: stripping
// blanks the inside of every string, so an import line reads as empty when the
// literal is the only thing on it, and a table of literals would count as
// nothing at all.
func (s *source) codeLines() int {
	count := 0

	for i := range s.read {
		if s.read[i].comment || strings.TrimSpace(s.line(i)) == "" {
			continue
		}
		count++
	}

	return count
}

// generated reports the marker a tool writes at the top of a file it owns.
//
// The convention is one line matching "^// Code generated .* DO NOT EDIT\.$"
// before the package clause, the marker every tool in the toolchain looks
// for and what this looks for.
func (s *source) generated() bool {
	for i := 0; i < s.len(); i++ {
		line := strings.TrimSpace(s.line(i))
		if strings.HasPrefix(line, "package ") {
			return false
		}
		if strings.HasPrefix(line, "// Code generated ") && strings.HasSuffix(line, " DO NOT EDIT.") {
			return true
		}
	}

	return false
}
