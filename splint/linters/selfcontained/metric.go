package selfcontained

// Metric is what the linter measured on one file.
//
// It is per file because scope is a property of a file: the package is where
// the names resolve, and the file is what a reader opens and what a compiler
// is handed one of.
type Metric struct {
	// Symbols is what the file declares, and Coupled is how many of those
	// reach a name declared in another file of the same package.
	Symbols int `json:"Symbols" yaml:"Symbols"`
	Coupled int `json:"Coupled" yaml:"Coupled"`

	// Types and Funcs are how many of each the file declares, and Self is how
	// many of those reach nothing outside it.
	Types     int `json:"Types" yaml:"Types"`
	TypesSelf int `json:"TypesSelf" yaml:"TypesSelf"`
	Funcs     int `json:"Funcs" yaml:"Funcs"`
	FuncsSelf int `json:"FuncsSelf" yaml:"FuncsSelf"`

	// Test reports a file the toolchain compiles into the test binary. A test
	// file reaches what it tests, so the two are counted apart.
	Test bool `json:"Test,omitempty" yaml:"Test,omitempty"`
}

// Coupling is the share of the file's symbols that reach another file, and is
// zero for a file that declares nothing.
func (m Metric) Coupling() float64 {
	if m.Symbols == 0 {
		return 0
	}
	return float64(m.Coupled) / float64(m.Symbols) * 100
}

// add folds one file's counts into another, which is how a package is counted
// from the files it is made of.
func (m *Metric) add(in Metric) {
	m.Symbols += in.Symbols
	m.Coupled += in.Coupled
	m.Types += in.Types
	m.TypesSelf += in.TypesSelf
	m.Funcs += in.Funcs
	m.FuncsSelf += in.FuncsSelf
}
