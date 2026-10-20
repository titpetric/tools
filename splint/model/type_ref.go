package model

import (
	"strings"
)

// TypeRef aims to trim a type name to a reference type.
func TypeRef(name string) string {
	// trim variadic arg
	if strings.HasPrefix(name, "...") {
		name = name[3:]
	}

	// slice, array and map value
	// for nesting, this is a hack
	if strings.HasPrefix(name, "[") || strings.HasPrefix(name, "map[") {
		name = strings.SplitN(name, "]", 2)[1]
	}

	// A generic instantiation names a type and its type arguments, and the
	// reference is the type alone: a method on *Stack[T] belongs to Stack,
	// and TestStack_Push tests it.
	if open := strings.IndexByte(name, '['); open >= 0 && strings.HasSuffix(name, "]") {
		name = name[:open]
	}

	// deref pointers
	return strings.TrimPrefix(name, "*")
}
