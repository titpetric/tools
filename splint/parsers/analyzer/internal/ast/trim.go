package ast

import (
	"go/ast"
	"strings"
)

// TrimSpace is the text of a comment group, trimmed, and empty for none.
func TrimSpace(in *ast.CommentGroup) string {
	if in != nil {
		return strings.TrimSpace(in.Text())
	}
	return ""
}
