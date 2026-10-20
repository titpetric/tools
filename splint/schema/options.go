package schema

// Options holds the settings of a conversion.
type Options struct {
	// StripPrefix are package prefixes to take off a definition name, so a
	// schema does not repeat the module path in every reference.
	StripPrefix []string
}
