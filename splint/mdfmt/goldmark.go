package mdfmt

import (
	"github.com/Kunde21/markdownfmt/v3/markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// This file holds every use of goldmark. One file of the package names the
// dependency. A reader looking for what builds the format finds it here.

// The renderer does not round-trip a code span whose content holds backticks.
//
// A document demonstrating backtick usage writes one: the delimiter is two
// backticks and the content between them is a third-and-fourth pair around a
// name. The renderer drops the inner pair. The sentence that
// demonstrated the backticks demonstrates nothing.
//
// mdox has the same fault from the same renderer, one major version back. It
// flattens the delimiter where this widens it, and loses the same inner pair.
//
// A formatter exists to settle what nobody should argue about. Rewriting what
// a document says is not that. Format answers ErrCodeSpan and leaves the file
// where it stands.

// reader parses without rendering. Format reads the code spans of a document
// back through it to compare them.
var reader = goldmark.New(goldmark.WithExtensions(extension.GFM))

// renderer builds the formatter. goldmark holds no state between conversions
// and a package level value would serve. A function keeps the options
// together.
func renderer(opts Options) goldmark.Markdown {
	render := markdown.NewRenderer()

	settings := []markdown.Option{markdown.WithCodeFormatters(markdown.GoCodeFormatter)}
	if opts.SoftWraps {
		settings = append(settings, markdown.WithSoftWraps())
	}
	render.AddMarkdownOptions(settings...)

	return goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAttribute()),
		goldmark.WithRenderer(render),
	)
}

// parse reads a document into its tree.
func parse(source []byte) ast.Node {
	return reader.Parser().Parse(text.NewReader(source))
}

// codeSpans are the contents of every code span of a document, in order.
func codeSpans(source []byte) []string {
	var found []string

	root := parse(source)
	ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		span, ok := node.(*ast.CodeSpan)
		if !ok || !entering {
			return ast.WalkContinue, nil
		}
		found = append(found, string(span.Text(source)))
		return ast.WalkSkipChildren, nil
	})

	return found
}
