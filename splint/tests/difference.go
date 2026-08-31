package tests

import "fmt"

// Difference is one place two values disagree.
type Difference struct {
	// Path names the value, as a dotted path into the document with array
	// indices in brackets: "Funcs[Open].Complexity.Cognitive".
	Path string

	// Left and Right are the two values, rendered for a reader.
	Left  string
	Right string
}

// String reads the difference as one line.
func (d Difference) String() string {
	return fmt.Sprintf("%s: %s != %s", d.Path, clip(d.Left), clip(d.Right))
}
