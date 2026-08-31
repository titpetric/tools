package simpleparser

import (
	"strings"
	"testing"
)

// TestStrip covers what the stripped view is for: a brace, a keyword or a
// backtick that is inside a string or a comment must not be read as code.
//
// The assertion is on the shape rather than on a column count. Every byte that
// is not code becomes a space and the length never changes, which is what lets
// a position in the stripped line be used against the line as written.
func TestStrip(t *testing.T) {
	tests := []struct {
		line string
		code string
	}{
		{line: `s := "}"`, code: "s :="},
		{line: "s := `func f() {`", code: "s :="},
		{line: `c := '}'`, code: "c :="},
		{line: `x := 1 // } func`, code: "x := 1"},
		{line: `x := 1 /* } */ + 2`, code: "x := 1 + 2"},
		{line: "\tName string `json:\"a\"`", code: "Name string"},
		{line: "plain code", code: "plain code"},
	}

	for _, test := range tests {
		got, _, _ := strip(test.line, false)

		if len(got) != len(test.line) {
			t.Errorf("strip(%q) returned %d bytes for %d, which breaks the column mapping",
				test.line, len(got), len(test.line))
		}
		if fields := strings.Join(strings.Fields(got), " "); fields != test.code {
			t.Errorf("strip(%q) left %q, want the code %q", test.line, fields, test.code)
		}
		// Nothing but code survives, so no quote, tick or slash is left.
		if strings.ContainsAny(got, "`\"'") || strings.Contains(got, "//") {
			t.Errorf("strip(%q) = %q, which still holds a literal or a comment", test.line, got)
		}
	}
}

func TestStripBlockCommentAcrossLines(t *testing.T) {
	first, _, open := strip("code /* start", false)
	if !open {
		t.Error("strip() closed a block comment that was left open")
	}
	if strings.TrimSpace(first) != "code" {
		t.Errorf("strip() = %q, want the code before the comment", first)
	}

	second, _, open := strip("still comment */ code", true)
	if open {
		t.Error("strip() left a block comment open past its end")
	}
	if strings.TrimSpace(second) != "code" {
		t.Errorf("strip() = %q, want the code after the comment", second)
	}
}

func TestSourceSplit(t *testing.T) {
	// A backtick inside a comment is not a struct tag, which is the whole
	// reason the comment position is recorded.
	src := newSource("x.go", []byte("\tParent string // `extends Name`, for printing\n"))

	code, comment := src.split(0)
	if code != "\tParent string " {
		t.Errorf("split() code = %q", code)
	}
	if comment != "// `extends Name`, for printing" {
		t.Errorf("split() comment = %q", comment)
	}
}

func TestSourceText(t *testing.T) {
	src := newSource("x.go", []byte("one\ntwo\nthree\n"))

	if got := src.text(0, 1); got != "one\ntwo" {
		t.Errorf("text(0, 1) = %q", got)
	}
	// A range past the end is clipped rather than refused.
	if got := src.text(2, 99); got != "three\n" {
		t.Errorf("text(2, 99) = %q", got)
	}
	if got := src.text(2, 1); got != "" {
		t.Errorf("text(2, 1) = %q, want it empty", got)
	}
}
