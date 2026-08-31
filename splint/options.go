package splint

// Options is what a parser is asked for. A caller builds one of these and
// hands it to whichever parser it imports; nothing else differs between them.
type Options struct {
	// SourcePath is the directory the parse is rooted at.
	SourcePath string

	// Pattern is "." for the package in SourcePath and "./..." for everything
	// below it, which is the only distinction the parsers make.
	Pattern string

	// IncludeTests keeps the test files and the test packages.
	IncludeTests bool

	// IncludeSources keeps the source of every declaration, which is most of
	// the size of a document and the whole of what a restore needs.
	IncludeSources bool

	// Verbose asks the parser to say what it is doing.
	Verbose bool
}

// NewOptions returns the defaults: the current directory, one package, no
// tests and no sources.
func NewOptions() Options {
	return Options{
		SourcePath: ".",
		Pattern:    ".",
	}
}

// Recursive reports whether the pattern reaches below the source path.
func (o Options) Recursive() bool {
	return o.Pattern == "./..."
}
