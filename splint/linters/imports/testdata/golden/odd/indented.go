// Package odd writes its import blocks in shapes gofmt would not.
package odd

import (
	"fmt"
	"strings"
)

// Indented closes its block on an indented paren. Reading that as the end of
// the file is how a rewrite comes to replace the whole file with a block.
func Indented() string { return fmt.Sprint(strings.TrimSpace("x")) }
