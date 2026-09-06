package dangle

import (
	"strings"
	"fmt"
	// A note that belongs to no import. A formatter may reorder a file and
	// may not lose a line of it.
)

// Dangle reaches both imports.
func Dangle() string { return fmt.Sprint(strings.TrimSpace("x")) }
