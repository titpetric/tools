package model

import (
	"strings"
)

// Declaration holds information about a go symbol.
type Declaration struct {
	Kind DeclarationKind `json:"Kind" yaml:"Kind"`
	Type string          `json:"Type,omitempty" yaml:"Type,omitempty"`

	File string `json:"File" yaml:"File"`
	Line int    `json:"Line,omitempty" yaml:"Line,omitempty"`

	SelfContained bool `json:"SelfContained" yaml:"SelfContained"`

	// This is not encoded to json, it's computed on load.
	Imports []string `json:"-" yaml:"-"`

	References StringSet `json:"References,omitempty" yaml:"References,omitempty"`
	Globals    StringSet `json:"Globals,omitempty" yaml:"Globals,omitempty"`

	Doc string `json:"Doc,omitempty" yaml:"Doc,omitempty"`

	Name     string   `json:"Name,omitempty" yaml:"Name,omitempty"`
	Names    []string `json:"Names,omitempty" yaml:"Names,omitempty"`
	Receiver string   `json:"Receiver,omitempty" yaml:"Receiver,omitempty"`

	Fields FieldList `json:"Fields,omitempty" yaml:"Fields,omitempty"`

	Arguments []string `json:"Arguments,omitempty" yaml:"Arguments,omitempty"`
	Returns   []string `json:"Returns,omitempty" yaml:"Returns,omitempty"`

	Signature string `json:"Signature,omitempty" yaml:"Signature,omitempty"`
	Source    string `json:"Source,omitempty" yaml:"Source,omitempty"`

	Complexity *Complexity `json:"Complexity,omitempty" yaml:"Complexity,omitempty"`
}

func (f *Declaration) IsTestScope() bool {
	return strings.HasSuffix(f.File, "_test.go")
}

func (f *Declaration) HasReceiver() bool {
	return f.Receiver != ""
}

func (f *Declaration) Ref(pkg *Package) []Ref {
	receiver := f.ReceiverTypeRef()
	result := make([]Ref, 0, len(f.Names)+1)
	for _, name := range f.Names {
		result = append(result, Ref{
			Package:  pkg,
			Name:     name,
			Receiver: receiver,
		})
	}

	result = append(result, Ref{
		Package:  pkg,
		Name:     f.Name,
		Receiver: receiver,
	})

	return result
}

func (f *Declaration) TypeRef() string {
	return TypeRef(f.Type)
}

func (f *Declaration) ReceiverTypeRef() string {
	return TypeRef(f.Receiver)
}

func (d *Declaration) GetNames() []string {
	if len(d.Names) > 0 {
		return d.Names
	}
	return []string{d.Name}
}

func (d *Declaration) HasName(find string) bool {
	for _, name := range d.Names {
		if name == find {
			return true
		}
	}
	return d.Name == find
}

func (d *Declaration) IsExported() bool {
	if d.Receiver != "" && !isExported(TypeRef(d.Receiver)) {
		return false
	}

	for _, name := range d.Names {
		if isExported(name) {
			return true
		}
	}
	return isExported(d.Name)
}

func (d *Declaration) Equal(in *Declaration) bool {
	if d.File == in.File && d.Kind == in.Kind && d.Name == in.Name {
		return true
	}
	return false
}

func (d *Declaration) Keys() []string {
	trimPath := "*."
	if d.Name != "" {
		return []string{
			strings.Trim(d.Receiver+"."+d.Name, trimPath),
		}
	}
	if len(d.Names) != 0 {
		result := make([]string, len(d.Names))
		for k, v := range d.Names {
			result[k] = strings.Trim(d.Receiver+"."+v, trimPath)
		}
		return result
	}
	return nil
}

// Symbol names the declaration the way a reader refers to it: the receiver and
// the name for a method, the name alone for everything else.
func (d *Declaration) Symbol() string {
	if d.Receiver != "" {
		return TypeRef(d.Receiver) + "." + d.Name
	}
	if d.Name == "" && len(d.Names) > 0 {
		return d.Names[0]
	}
	return d.Name
}

// Position returns where the declaration is, relative to the root of the
// parse. The declaration records the base filename and the package records the
// directory, so neither says on its own where a reader should look.
func (d *Declaration) Position(pkg Package) Position {
	dir := strings.TrimPrefix(strings.TrimPrefix(pkg.Path, "."), "/")
	file := d.File
	if dir != "" {
		file = dir + "/" + file
	}
	return Position{Package: pkg.Package, File: file, Line: d.Line}
}
