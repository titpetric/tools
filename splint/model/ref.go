package model

import (
	"fmt"
)

// Ref names one symbol of a package, with its receiver for a method.
type Ref struct {
	Package  *Package `json:"Package" yaml:"Package"`
	Receiver string   `json:"Receiver" yaml:"Receiver"`
	Name     string   `json:"Name" yaml:"Name"`
}

// String is pkg.Receiver.Name for a method,
// and pkg.Name otherwise.
func (r Ref) String() string {
	if r.Receiver != "" {
		return fmt.Sprintf("%s.%s.%s", r.Package.Name(), TypeRef(r.Receiver), TypeRef(r.Name))
	}
	return fmt.Sprintf("%s.%s", r.Package.Name(), TypeRef(r.Name))
}
