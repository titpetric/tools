package ast

import (
	"go/ast"
	"go/printer"
	"go/token"
	"io"
)

// CommentedNode pairs a node with the comments of its file for printing.
func CommentedNode(file *ast.File, node any) *printer.CommentedNode {
	return &printer.CommentedNode{
		Node:     node,
		Comments: file.Comments,
	}
}

// PrintSource prints a commented node as Go source.
func PrintSource(out io.Writer, fset *token.FileSet, val *printer.CommentedNode) error {
	return printer.Fprint(out, fset, val)
}
