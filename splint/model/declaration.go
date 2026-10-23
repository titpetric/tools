package model

import (
	"strings"
)

// Declaration holds information about a go symbol. A declaration records its
// base filename and its package the directory, so Position joins the two.
type Declaration struct {
	Kind DeclarationKind `json:"Kind" yaml:"Kind"`
	Type string          `json:"Type,omitempty" yaml:"Type,omitempty"`

	File string `json:"File" yaml:"File"`
	Line int    `json:"Line,omitempty" yaml:"Line,omitempty"`

	// Encoding to json skips this field; loading computes it.
	Imports []string `json:"-" yaml:"-"`

	// References are the packages the declaration reaches through an import of
	// its own file, keyed by the name the import spells: json -> Encode. An
	// import resolves within the scope of one file, so the parse records the
	// same symbol reached from two files on both.
	References StringSet `json:"References,omitempty" yaml:"References,omitempty"`

	// Globals are the package level names the declaration reaches that its own
	// file neither declares nor binds. A bare identifier is a key with no
	// symbols; a selector on a package level variable is a key with the name
	// after the dot.
	//
	// This field says how far a declaration reaches inside its own package: a
	// declaration with none of them names only its imports and what is beside
	// it in the file.
	Globals StringSet `json:"Globals,omitempty" yaml:"Globals,omitempty"`

	Doc string `json:"Doc,omitempty" yaml:"Doc,omitempty"`

	Name     string   `json:"Name,omitempty" yaml:"Name,omitempty"`
	Names    []string `json:"Names,omitempty" yaml:"Names,omitempty"`
	Receiver string   `json:"Receiver,omitempty" yaml:"Receiver,omitempty"`

	Fields FieldList `json:"Fields,omitempty" yaml:"Fields,omitempty"`

	Arguments []string `json:"Arguments,omitempty" yaml:"Arguments,omitempty"`
	Returns   []string `json:"Returns,omitempty" yaml:"Returns,omitempty"`

	// ArgumentCount is how many parameters a caller passes. Arguments holds
	// the distinct types and no duplicate, for a reader of the dependency
	// model; its length can differ from the length of the parameter list.
	// date(year int, month time.Month, day, hour, min, sec int) has seven
	// parameters and two entries in Arguments, and a rule about the order of
	// two arguments has to be able to tell those apart.
	ArgumentCount int `json:"ArgumentCount,omitempty" yaml:"ArgumentCount,omitempty"`

	// TypeParams are the names a generic func binds between its name and its
	// parameter list: Eval[T any] binds T. A name in here is a stand-in the
	// caller fills, and no type the package declares; that lets a reader of
	// the model tell Exec[T]() (T, error) from a constructor.
	TypeParams []string `json:"TypeParams,omitempty" yaml:"TypeParams,omitempty"`

	Signature string `json:"Signature,omitempty" yaml:"Signature,omitempty"`
	Source    string `json:"Source,omitempty" yaml:"Source,omitempty"`

	Complexity *Complexity `json:"Complexity,omitempty" yaml:"Complexity,omitempty"`

	// Literals are the string literals a declaration of a test file writes,
	// unquoted, sorted and deduplicated: the DSN a test opens, the command
	// line it runs. The parse fills them when its options include the test
	// facts, and only in test files, where a literal says what the test needs.
	Literals []string `json:"Literals,omitempty" yaml:"Literals,omitempty"`

	// Calls are the functions and methods on concrete types a function body
	// calls, keyed by the import path of the package declaring them, with
	// the symbol as Func or Type.Method: "os/exec": {"Command"}. A call
	// through an interface or a func value names no declaration and is not
	// recorded. The ast parser fills them when its options include the test
	// facts; the simple parser resolves no types and leaves them empty.
	Calls StringSet `json:"Calls,omitempty" yaml:"Calls,omitempty"`
}

// IsTestScope reports whether the declaration sits in a _test.go file.
func (f *Declaration) IsTestScope() bool {
	return strings.HasSuffix(f.File, "_test.go")
}

// HasReceiver reports whether the declaration is a method.
func (f *Declaration) HasReceiver() bool {
	return f.Receiver != ""
}

// HasTypeParam reports whether name is one of the declaration's type
// parameters, and false for a declared type.
func (f *Declaration) HasTypeParam(name string) bool {
	for _, param := range f.TypeParams {
		if param == name {
			return true
		}
	}
	return false
}

// Ref is a reference to each name the declaration declares.
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

// TypeRef is the declared type without pointer or slice markers.
func (f *Declaration) TypeRef() string {
	return TypeRef(f.Type)
}

// ReceiverTypeRef is the receiver type without its pointer.
func (f *Declaration) ReceiverTypeRef() string {
	return TypeRef(f.Receiver)
}

// GetNames lists the names of a block, or the single name.
func (d *Declaration) GetNames() []string {
	if len(d.Names) > 0 {
		return d.Names
	}
	return []string{d.Name}
}

// HasName reports a name among those declared,
// or a match of the single name.
func (d *Declaration) HasName(find string) bool {
	for _, name := range d.Names {
		if name == find {
			return true
		}
	}
	return d.Name == find
}

// IsExported is false for a method on an unexported type,
// true when a block exports a name,
// and reports an exported single name otherwise.
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

// Equal matches the file, kind and name.
func (d *Declaration) Equal(in *Declaration) bool {
	if d.File == in.File && d.Kind == in.Kind && d.Name == in.Name {
		return true
	}
	return false
}

// Keys are the receiver-qualified names,
// one per declared name,
// or nil for a declaration with no name.
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

// Position returns where the declaration is, relative to the root of the parse.
func (d *Declaration) Position(pkg Package) Position {
	dir := strings.TrimPrefix(strings.TrimPrefix(pkg.Path, "."), "/")
	file := d.File
	if dir != "" {
		file = dir + "/" + file
	}
	return Position{Package: pkg.Package, File: file, Line: d.Line}
}
