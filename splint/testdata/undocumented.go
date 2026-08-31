package fixture

// This comment does not open on the symbol it documents, which is what the
// godoc check reports.
type Thing struct {
	Value int
}

func Undocumented() int {
	return 0
}

// Unpunctuated is documented and does not end in punctuation
func Unpunctuated() int {
	return 0
}
