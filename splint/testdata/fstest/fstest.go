// Package fstest is named the way the standard library names a package of
// test code, which excuses the test files that are nobody's counterpart.
package fstest

// Fake returns what a test would stand in with.
func Fake() string {
	return "fake"
}
