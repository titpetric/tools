package model

import (
	"fmt"
	"strings"
)

// Package holds go package information.
type Package struct {
	// ID is the ID of the package as the parser identifies it, which for the
	// ast parser is what x/tools loads it under.
	ID string `json:"ID" yaml:"ID"`
	// Package is the name of the package.
	Package string `json:"Package" yaml:"Package"`
	// ImportPath contains the import path (github...).
	ImportPath string `json:"ImportPath" yaml:"ImportPath"`
	// Path is sanitized to contain the relative location (folder).
	Path string `json:"Path" yaml:"Path"`
	// TestPackage is true if this is a test package.
	TestPackage bool `json:"TestPackage" yaml:"TestPackage"`

	// Complexity to collect test coverage on package.
	Complexity *Complexity `json:"Complexity,omitempty" yaml:"Complexity,omitempty"`
}

func (p Package) Name() string {
	return p.Package
}

func (p Package) String() string {
	return fmt.Sprintf("package=%s import_path=%s path=%s test_package=%v", p.Package, p.ImportPath, p.Path, p.TestPackage)
}

func (p Package) Equal(in Package) bool {
	return p.ImportPath == in.ImportPath
}

func (p Package) Namespace(suffix string) string {
	var namespace string
	packagePath := strings.Trim(p.Path, "./")
	if packagePath != "" {
		namespace = strings.ReplaceAll(packagePath, "/", ".")
	}
	if namespace == "" {
		namespace = p.Package
	}
	return namespace + suffix
}
