package simpleparser

import "testing"

// TestDocAbove covers what go/ast's CommentGroup.Text does, which is what the
// doc has to read like for the two parsers to agree.
func TestDocAbove(t *testing.T) {
	src := newSource("x.go", []byte(`// Open reads a file.
//
// An indented block is an example:
//
//	f, err := Open("x")
//
//go:noinline
func Open() {}
`))

	got := docAbove(src, 7)
	want := "Open reads a file.\n\nAn indented block is an example:\n\n\tf, err := Open(\"x\")"
	if got != want {
		t.Errorf("docAbove() = %q, want %q", got, want)
	}
}

func TestTrimMarker(t *testing.T) {
	tests := map[string]string{
		"// text":      "text",
		"//text":       "text",
		"//  text":     " text",
		"//\tindented": "\tindented",
		"//":           "",
		"/* text */":   "text",
		"\t// spaced ": "spaced",
	}

	for line, want := range tests {
		if got := trimMarker(line); got != want {
			t.Errorf("trimMarker(%q) = %q, want %q", line, got, want)
		}
	}
}

func TestIsDirective(t *testing.T) {
	tests := map[string]bool{
		"//go:embed all:assets":  true,
		"//go:generate stringer": true,
		"//line x.go:12":         true,
		"//export Name":          false,
		"// go:embed":            false,
		"// a normal comment":    false,
		"//nolint":               false,
		"//":                     false,
	}

	for line, want := range tests {
		if got := isDirective(line); got != want {
			t.Errorf("isDirective(%q) = %v, want %v", line, got, want)
		}
	}
}

func TestBlockEnd(t *testing.T) {
	src := newSource("x.go", []byte("var (\n\ta = 1\n\tb = \")\"\n)\nnext\n"))

	// The paren inside the string does not close the block.
	if got := blockEnd(src, 0, ')'); got != 3 {
		t.Errorf("blockEnd() = %d, want 3", got)
	}

	// A block that is never closed ends where the file does, which is what
	// makes the parser tolerant of source that does not compile.
	open := newSource("x.go", []byte("var (\n\ta = 1\n"))
	if got := blockEnd(open, 0, ')'); got != open.len()-1 {
		t.Errorf("blockEnd() = %d, want the last line %d", got, open.len()-1)
	}
}
