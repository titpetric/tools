package model

import (
	"fmt"
)

type Ref struct {
	Package  *Package `json:"Package" yaml:"Package"`
	Receiver string   `json:"Receiver" yaml:"Receiver"`
	Name     string   `json:"Name" yaml:"Name"`
}

func (r Ref) String() string {
	if r.Receiver != "" {
		return fmt.Sprintf("%s.%s.%s", r.Package.Name(), TypeRef(r.Receiver), TypeRef(r.Name))
	}
	return fmt.Sprintf("%s.%s", r.Package.Name(), TypeRef(r.Name))
}
