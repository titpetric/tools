package visibility

// Metric is what the linter counted in one package.
type Metric struct {
	// ExportedTypes and InternalTypes count declared types by the case of the
	// name they are declared under.
	ExportedTypes int `json:"ExportedTypes" yaml:"ExportedTypes"`
	InternalTypes int `json:"InternalTypes" yaml:"InternalTypes"`

	// ExportedFuncs and InternalFuncs count funcs and methods alike, so a
	// method named serveHTTP is internal the way a free function of that name
	// is.
	ExportedFuncs int `json:"ExportedFuncs" yaml:"ExportedFuncs"`
	InternalFuncs int `json:"InternalFuncs" yaml:"InternalFuncs"`

	// InternalLines is the code behind the internal funcs, and Lines is the
	// code of the package, both counted with blanks and comments left out.
	InternalLines int `json:"InternalLines" yaml:"InternalLines"`
	Lines         int `json:"Lines" yaml:"Lines"`
}

// Exported is the symbols a consumer of the package can reach, types and funcs
// together.
func (m Metric) Exported() int {
	return m.ExportedTypes + m.ExportedFuncs
}

// Internal is the symbols the package keeps to itself.
func (m Metric) Internal() int {
	return m.InternalTypes + m.InternalFuncs
}

// Ratio is the share of the package's code that sits behind its internal
// funcs, as a percentage. A package holding no code has no share to take.
func (m Metric) Ratio() float64 {
	if m.Lines == 0 {
		return 0
	}
	return float64(m.InternalLines) / float64(m.Lines) * 100
}
