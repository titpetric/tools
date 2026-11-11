package model

// ProseFile is one markdown file as a prose rule reads it.
//
// A markdown file declares nothing, which keeps it out of Packages. It holds
// paragraphs. A rule about writing reads a paragraph the way it reads a doc
// comment.
type ProseFile struct {
	// Path is the file, relative to the root of the parse.
	Path string `json:"Path" yaml:"Path"`

	// Blocks are the paragraphs, in the order the file writes them.
	Blocks []ProseBlock `json:"Blocks,omitempty" yaml:"Blocks,omitempty"`
}

// ProseBlock is one paragraph of running text.
//
// Fences, tables and headings stay out. A fence holds code, a table holds
// measurements, a heading labels a section. A rule about sentences that read
// the three of them reports noise.
type ProseBlock struct {
	// Line is where the paragraph starts. A finding then names a place.
	Line int `json:"Line" yaml:"Line"`

	// Text is the paragraph as one line. A break the author put inside a
	// paragraph means nothing in markdown, and joining them counts a wrapped
	// phrase once.
	Text string `json:"Text" yaml:"Text"`

	// Comment reports a block a configuration file writes as a comment rather
	// than as a value. A markdown paragraph is neither.
	Comment bool `json:"Comment,omitempty" yaml:"Comment,omitempty"`

	// Depth is how deep in a configuration file's tree the node the block
	// attaches to sits. A top-level key is zero, and a markdown paragraph is.
	Depth int `json:"Depth,omitempty" yaml:"Depth,omitempty"`
}

// AddProse records one file's paragraphs, unless it holds none.
func (d *DocumentRoot) AddProse(file *ProseFile) {
	if file == nil || len(file.Blocks) == 0 {
		return
	}
	d.Prose = append(d.Prose, file)
}
