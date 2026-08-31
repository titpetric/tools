package model

import "sort"

// LintMetrics is what a linter measured, keyed by what it measured it on.
//
// A linter fills one of the two, or neither. The values are the linter's own
// metric type: what a measurement means is the linter's business, and the
// framework only carries it as far as the encoder.
type LintMetrics struct {
	// Files is keyed by the path a Position names, "frontend/view/page.go".
	Files map[string]any `json:"Files,omitempty" yaml:"Files,omitempty"`

	// Packages is keyed by the import path.
	Packages map[string]any `json:"Packages,omitempty" yaml:"Packages,omitempty"`
}

// NewLintMetrics returns metrics ready to be filled.
func NewLintMetrics() LintMetrics {
	return LintMetrics{
		Files:    map[string]any{},
		Packages: map[string]any{},
	}
}

// AddFile records what was measured on one file.
func (m *LintMetrics) AddFile(path string, value any) {
	if m.Files == nil {
		m.Files = map[string]any{}
	}
	m.Files[path] = value
}

// AddPackage records what was measured on one package.
func (m *LintMetrics) AddPackage(importPath string, value any) {
	if m.Packages == nil {
		m.Packages = map[string]any{}
	}
	m.Packages[importPath] = value
}

// Empty reports metrics holding nothing, which is a linter that measures
// nothing rather than one that measured nothing.
func (m LintMetrics) Empty() bool {
	return len(m.Files) == 0 && len(m.Packages) == 0
}

// FileKeys and PackageKeys are the keys in order, so a rendering of the
// metrics reads the same every time.
func (m LintMetrics) FileKeys() []string    { return sortedKeys(m.Files) }
func (m LintMetrics) PackageKeys() []string { return sortedKeys(m.Packages) }

// sortedKeys returns the keys of a map in order.
func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
