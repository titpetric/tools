package schema

// Options is what a conversion is asked for.
type Options struct {
	// StripPrefix are package prefixes to take off a definition name, so a
	// schema does not repeat the module path in every reference.
	StripPrefix []string
}
