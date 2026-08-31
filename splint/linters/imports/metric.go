package imports

// Metric is what the linter counted in one package.
type Metric struct {
	// Files is how many files the package is made of, and Names how many
	// short names their imports are reached by.
	Files int `json:"Files" yaml:"Files"`
	Names int `json:"Names" yaml:"Names"`

	// Collisions is how many of those names mean two things.
	Collisions int `json:"Collisions" yaml:"Collisions"`
}
