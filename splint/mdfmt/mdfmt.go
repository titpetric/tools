// Package mdfmt formats markdown the one way the tree writes it.
//
// The format is whatever goldmark parses and markdownfmt renders, with Go
// code blocks run through gofmt. No setting changes it. Two documents under
// one rule read the same. A repository arguing about list markers argues
// about a list marker.
package mdfmt

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// skipDirs are the trees whose markdown belongs to somebody else.
var skipDirs = map[string]bool{
	"vendor":       true,
	"node_modules": true,
	".git":         true,
	"testdata":     true,
}

// Options are the house rule for how a document comes back.
//
// There is one field. It answers the one question two trees disagree about,
// and the format settles the rest.
type Options struct {
	// SoftWraps keeps the line breaks the author put inside a paragraph.
	// Off, a paragraph comes back as one line.
	SoftWraps bool
}

// Format returns the document as the rule writes it.
func Format(opts Options, source []byte) ([]byte, error) {
	front, body := frontMatter(source)

	// A file of nothing but front matter is the data of a page a generator
	// renders. It has no prose. The blank line that separates a block
	// from a body separates it from nothing.
	if len(bytes.TrimSpace(body)) == 0 {
		return front, nil
	}

	out := &bytes.Buffer{}
	if len(front) > 0 {
		out.Write(front)
		out.WriteByte('\n')
	}
	if err := renderer(opts).Convert(body, out); err != nil {
		return nil, err
	}

	formatted := out.Bytes()
	if !sameCodeSpans(source, formatted) {
		return nil, ErrCodeSpan
	}

	return formatted, nil
}

// frontMatter lifts a leading "---" block off the front of a document and
// answers it with the body that follows.
//
// The renderer otherwise reads the delimiters as a setext heading, turns the
// whole block into a level two heading and loses the keys.
//
// The block passes through as the file holds it. A formatter that reflows YAML
// no reader sees has changed what nobody asked it to.
func frontMatter(source []byte) (front, body []byte) {
	const fence = "---\n"

	if !bytes.HasPrefix(source, []byte(fence)) {
		return nil, source
	}

	rest := source[len(fence):]
	end := bytes.Index(rest, []byte("\n"+fence))
	if end < 0 {
		return nil, source
	}

	cut := len(fence) + end + len(fence) + 1
	return source[:cut], bytes.TrimLeft(source[cut:], "\n")
}

// FormatFile rewrites one file and reports whether the file changed.
//
// A file already in the format keeps its modification time: an editor and a
// build cache both read it, and rewriting bytes that did not change wakes
// both for nothing.
func FormatFile(opts Options, path string) (bool, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	formatted, err := Format(opts, source)
	if errors.Is(err, ErrCodeSpan) {
		return false, err
	}
	if err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	if bytes.Equal(source, formatted) {
		return false, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return true, os.WriteFile(path, formatted, info.Mode().Perm())
}

// Files are the markdown files under a root, in the order the walk reaches
// them. A root naming one file answers that file.
//
// The walk drops a file a Go program embeds. See embedded.go.
func Files(root string, recursive bool) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		if filepath.Ext(root) != ".md" {
			return nil, nil
		}
		return []string{root}, nil
	}

	var found []string
	cache := embedCache{}
	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			switch {
			case path == root:
				return nil
			case skipDirs[entry.Name()], entry.Name()[0] == '.':
				return fs.SkipDir
			case !recursive:
				return fs.SkipDir
			}
			return nil
		}

		if filepath.Ext(path) != ".md" {
			return nil
		}

		// A directive names its files relative to its own directory. A
		// pattern reaches below it. Every directory from the file up to the
		// root gets the question.
		for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
			if cache.embedded(dir, path) {
				return nil
			}
			if dir == root || dir == filepath.Dir(dir) {
				break
			}
		}

		found = append(found, path)
		return nil
	}

	if err := filepath.WalkDir(root, walk); err != nil {
		return nil, err
	}

	return found, nil
}
