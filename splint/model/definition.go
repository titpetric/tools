package model

import (
	"sort"
	"strings"
)

// Definition holds all symbols defined for a Package.
type Definition struct {
	Package `yaml:",inline"`

	Doc string `json:"Doc,omitempty" yaml:"Doc,omitempty"`

	// Module is the go.mod the package belongs to. It repeats on every
	// definition of the same module rather than living in one place, because
	// the model is a flat list of packages with no entry to hang it off, and
	// a definition that travels alone still has to say what it builds against.
	// It is nil for a package extracted from a tree holding no go.mod.
	Module *Module `json:"Module,omitempty" yaml:"Module,omitempty"`

	// Files are the files the package is made of, in name order, whether or
	// not they declare anything.
	Files FileList `json:"Files,omitempty" yaml:"Files,omitempty"`

	Imports   StringSet `json:"Imports,omitempty" yaml:"Imports,omitempty"`
	InitCount int       `json:"InitCount,omitempty" yaml:"InitCount,omitempty"`

	Types  DeclarationList `json:"Types,omitempty" yaml:"Types,omitempty"`
	Consts DeclarationList `json:"Consts,omitempty" yaml:"Consts,omitempty"`
	Vars   DeclarationList `json:"Vars,omitempty" yaml:"Vars,omitempty"`
	Funcs  DeclarationList `json:"Funcs,omitempty" yaml:"Funcs,omitempty"`
}

// DeclarationList merges all the declarations together.
func (d *Definition) DeclarationList() DeclarationList {
	allDecls := DeclarationList{}
	allDecls.Append(d.Consts...)
	allDecls.Append(d.Vars...)
	allDecls.Append(d.Types...)
	allDecls.Append(d.Funcs...)
	return allDecls
}

// Sort will sort the inner types so they have a stable order.
func (d *Definition) Sort() {
	sort.Slice(d.Files, func(i, j int) bool { return d.Files[i].Name < d.Files[j].Name })
	d.Types.Sort()
	d.Vars.Sort()
	d.Consts.Sort()
	d.Funcs.Sort()
}

func (d *Definition) Order() DeclarationList {
	count := len(d.Types) + len(d.Funcs) + len(d.Vars) + len(d.Consts)
	result := make([]*Declaration, 0, count)

	result = append(result, d.Types...)
	result = append(result, d.Funcs...)
	result = append(result, d.Vars...)
	result = append(result, d.Consts...)
	return DeclarationList(result)
}

func (d *Definition) ClearSource() {
	d.Types.ClearSource()
	d.Vars.ClearSource()
	d.Consts.ClearSource()
	d.Funcs.ClearSource()
}

func (d *Definition) ClearTestFiles() {
	for filename := range d.Imports {
		if strings.HasSuffix(filename, "_test.go") {
			delete(d.Imports, filename)
		}
	}
	d.Files = d.Files.Filter(func(f File) bool { return !f.Test })
	d.Types.ClearTestFiles()
	d.Vars.ClearTestFiles()
	d.Consts.ClearTestFiles()
	d.Funcs.ClearTestFiles()
}

func (d *Definition) ClearNonTestFiles() {
	for filename := range d.Imports {
		if !strings.HasSuffix(filename, "_test.go") {
			delete(d.Imports, filename)
		}
	}
	d.Files = d.Files.Filter(func(f File) bool { return f.Test })
	d.Types.ClearNonTestFiles()
	d.Vars.ClearNonTestFiles()
	d.Consts.ClearNonTestFiles()
	d.Funcs.ClearNonTestFiles()
}

func (d *Definition) Fill() {
	for _, decl := range d.Order() {
		decl.Imports = d.getImports(decl)
	}
}

func (d *Definition) Merge(in *Definition) {
	d.TestPackage = d.TestPackage || in.TestPackage

	// Both sides are the same package, so they are the same module. Only the
	// side that has one can contribute it.
	if d.Module == nil {
		d.Module = in.Module
	}

	for k, v := range in.Imports {
		d.Imports.Add(k, v...)
	}

	for _, file := range in.Files {
		if _, known := d.Files.Find(file.Name); !known {
			d.Files = append(d.Files, file)
		}
	}

	d.Types.AppendUnique(in.Types...)
	d.Funcs.AppendUnique(in.Funcs...)
	d.Vars.AppendUnique(in.Vars...)
	d.Consts.AppendUnique(in.Consts...)

	// this line causes Sort to be omitted from the
	// definitions :/ ... solved by adding the sort
	// in the AppendUnique above, but the Sort symbol
	// should not be omitted from Definition.

	// d.Sort()
}

func (d *Definition) getImports(decl *Declaration) []string {
	return d.Imports.Get(decl.File)
}
