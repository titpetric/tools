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

	// InternalLines is the code behind the internal funcs and Lines the code
	// behind all of them, both measured as the block a func occupies.
	//
	// Both sides are the same unit, which is what makes the one a share of the
	// other. Measuring the internal funcs by their block and the package by
	// its code lines counts blank lines and comments on one side of the
	// division and not the other, and a package that explains itself inside
	// its funcs then reads past a hundred percent.
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

// Ratio is the share of the package's functions that are private, as a
// percentage. A package holding no functions has no share to take.
func (m Metric) Ratio() float64 {
	if m.Lines == 0 {
		return 0
	}
	return float64(m.InternalLines) / float64(m.Lines) * 100
}
