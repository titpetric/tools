// Package cgo imports "C", which is not an import.
//
// The comment above it is the C source cgo compiles, and it is the preamble
// only while it sits directly above that import, so that declaration is left
// exactly where it is. Every other block in the file is formatted as usual.
package cgo

/*
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// Free is here to reach both imports.
func Free(p unsafe.Pointer) { C.free(p); fmt.Println("freed") }
