package ast

import (
	"go/ast"
	"go/printer"
	"go/token"
	"io"
)

func CommentedNode(file *ast.File, node any) *printer.CommentedNode {
	return &printer.CommentedNode{
		Node:     node,
		Comments: file.Comments,
	}
}

func PrintSource(out io.Writer, fset *token.FileSet, val *printer.CommentedNode) error {
	return printer.Fprint(out, fset, val)
}
