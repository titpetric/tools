package prose

import (
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// Block is one piece of running text and where it starts. A doc comment and a
// markdown paragraph both arrive as one of these. One rule then reads both.
type Block struct {
	File string
	Line int
	Text string

	// Prose marks a markdown paragraph. A paragraph runs as long as the
	// thought behind it. A doc comment answers to the symbol above it. No one
	// baseline judges both.
	Prose bool
}

// Corpus is every block a document holds.
type Corpus struct {
	Blocks []Block
}

// Docs is the corpus of doc comments alone, which the lengths compare against.
func (c *Corpus) Docs() *Corpus {
	out := &Corpus{}
	for _, block := range c.Blocks {
		if !block.Prose {
			out.Blocks = append(out.Blocks, block)
		}
	}
	return out
}

// FromDocument reads the doc comments and markdown paragraphs of one parse.
func FromDocument(root *model.DocumentRoot) *Corpus {
	corpus := &Corpus{}

	for _, def := range root.Packages {
		// A package comment has no declaration to hang off. One of the
		// package's files holds it and the document never says which. The
		// block takes the package path and no line, and Count prefers an
		// instance that names one.
		//
		// A test package shares the directory's package comment and writes
		// none itself. The ast parser copies it over and the quick
		// parser leaves it empty. Reading it would count one comment twice and
		// measure which parser ran.
		if !def.TestPackage {
			corpus.add(def.Path, 0, def.Doc, false)
		}
		for _, decl := range def.DeclarationList() {
			corpus.add(decl.File, decl.Line, decl.Doc, false)
		}
	}

	for _, file := range root.Prose {
		for _, block := range file.Blocks {
			corpus.add(file.Path, block.Line, block.Text, true)
		}
	}

	return corpus
}

// add records one block, joined into the sentence a reader reads.
//
// An indented line is an example, which a generated reference prints as code.
// It leaves the measure: a run of identifiers is not a sentence, and counting
// it reports the shape of the example.
func (c *Corpus) add(file string, line int, doc string, prose bool) {
	if strings.TrimSpace(doc) == "" {
		return
	}

	var kept []string
	for _, one := range strings.Split(doc, "\n") {
		if strings.HasPrefix(one, "\t") || strings.HasPrefix(one, "    ") {
			continue
		}
		if trimmed := strings.TrimSpace(one); trimmed != "" {
			kept = append(kept, trimmed)
		}
	}

	if text := strings.Join(kept, " "); text != "" {
		c.Blocks = append(c.Blocks, Block{File: file, Line: line, Text: text, Prose: prose})
	}
}
