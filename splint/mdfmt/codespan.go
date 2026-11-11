package mdfmt

import (
	"errors"
)

// ErrCodeSpan says the format would change what a code span says. The
// renderer's fault is in goldmark.go.
var ErrCodeSpan = errors.New("formatting would change the content of a code span")

// sameCodeSpans reports whether two documents say the same thing in every
// code span.
func sameCodeSpans(before, after []byte) bool {
	first, second := codeSpans(before), codeSpans(after)
	if len(first) != len(second) {
		return false
	}

	for i := range first {
		if first[i] != second[i] {
			return false
		}
	}

	return true
}
