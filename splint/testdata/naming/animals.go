// Package naming declares symbols in files not named for them, which is what
// the grouping check reports.
//
// The file is animals.go rather than naming.go on purpose. A symbol is also
// accepted in a file named for its package, so a type in naming.go would pass
// the check and the fixture would prove nothing.
package naming

// Elephant is declared in naming.go and wants to be in elephant.go.
type Elephant struct {
	Name string
}

// Giraffe is declared in naming.go as well, and two exported types in one file
// is what the check is about.
type Giraffe struct {
	Height int
}
