package dangle

// The first declaration keeps this comment: the block is written where this
// declaration was, so the comment above it is still above it.
import (
	"fmt"
	"strings"
	// This one is about a declaration that is written away. Left here it would
	// read as a comment about the function below.
)

// Merged reaches both.
func Merged() string { return fmt.Sprint(strings.TrimSpace("x")) }
