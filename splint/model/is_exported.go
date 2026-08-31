package model

import "unicode"

// isExported reports whether a declared name is visible outside its package,
// which in Go is whether it starts with an upper case letter.
//
// go/ast has this function, and the model does not import go/ast: the schema
// has to be readable by a parser that never builds a syntax tree.
func isExported(name string) bool {
	for _, r := range name {
		return unicode.IsUpper(r)
	}
	return false
}
