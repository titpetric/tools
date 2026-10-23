package splint

// Options holds the settings of a parse. A caller builds one of these and
// passes it to whichever parser it imports; nothing else differs between them.
type Options struct {
	// SourcePath is the root directory of the parse.
	SourcePath string

	// Pattern is "." for the package in SourcePath and "./..." for everything
	// below it, which is the only distinction the parsers make.
	Pattern string

	// IncludeTests keeps the test files and the test packages.
	IncludeTests bool

	// IncludeSources keeps the source of every declaration, which is most of
	// the size of a document and the whole of what a restore needs.
	IncludeSources bool

	// IncludeImports keeps the import declarations of every file as the file
	// writes them, and the names each file writes before a dot. The import
	// rules and the fixer read them; nothing else needs them, and a parse
	// without this setting does not pay for them.
	IncludeImports bool

	// IncludeTestFacts keeps what the integration rule reads of the test
	// files: the string literals every test declaration writes, and the test
	// files a build constraint keeps out of this build. It reads nothing
	// unless IncludeTests holds too.
	IncludeTestFacts bool

	// Verbose makes the parser report its progress.
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
