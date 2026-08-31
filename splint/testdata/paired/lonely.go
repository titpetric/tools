// This file has no lonely_test.go beside it, which the pairing check reports
// and the coverage check reports again for the symbol.
package paired

// Lonely is exported, undocumented in the way the godoc check wants, and
// tested by nobody
func Lonely() int {
	return 2
}
