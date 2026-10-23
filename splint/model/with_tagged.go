package model

import (
	"strings"
)

// WithTagged returns the document itself when it holds no tagged tests,
// and otherwise a view with the tests behind tag merged into their test packages.
func (d *DocumentRoot) WithTagged(tag string) *DocumentRoot {
	if d == nil || len(d.Tagged) == 0 {
		return d
	}

	out := *d
	out.Packages = append(DefinitionList(nil), d.Packages...)
	out.Tagged = nil

	for _, tagged := range d.Tagged {
		in := behind(tagged, tag)
		if in == nil {
			continue
		}

		found := -1
		for index, def := range out.Packages {
			if def != nil && def.TestPackage && def.Package.Path == in.Package.Path {
				found = index
				break
			}
		}
		if found < 0 {
			out.Packages = append(out.Packages, in)
			continue
		}

		merged := copyDefinition(out.Packages[found])
		merged.Merge(in)
		out.Packages[found] = merged
	}

	return &out
}

// BehindTag reports a constraint that holds only with the tag set: the tag
// appears in one of the expressions, and not negated.
func BehindTag(exprs []string, tag string) bool {
	for _, expr := range exprs {
		words := strings.FieldsFunc(expr, func(r rune) bool {
			return r == ' ' || r == '(' || r == ')' || r == '&' || r == '|'
		})
		for _, word := range words {
			if word == tag {
				return true
			}
		}
	}
	return false
}

// behind is the part of a tagged definition whose files sit behind the tag,
// and nil when none does.
func behind(def *Definition, tag string) *Definition {
	keep := func(file string) bool {
		return BehindTag(def.Excluded[file], tag)
	}

	out := *def
	out.Excluded = nil
	out.Files = def.Files.Filter(func(f File) bool { return keep(f.Name) })
	if len(out.Files) == 0 {
		return nil
	}

	out.Imports = StringSet{}
	for file, imports := range def.Imports {
		if keep(file) {
			out.Imports[file] = imports
		}
	}
	inFiles := func(decl *Declaration) bool { return keep(decl.File) }
	out.Types = def.Types.Filter(inFiles)
	out.Consts = def.Consts.Filter(inFiles)
	out.Vars = def.Vars.Filter(inFiles)
	out.Funcs = def.Funcs.Filter(inFiles)
	return &out
}

// copyDefinition copies a definition deep enough for Merge to add to it
// without touching the original.
func copyDefinition(def *Definition) *Definition {
	out := *def
	out.Files = append(FileList(nil), def.Files...)
	out.Imports = StringSet{}
	for file, imports := range def.Imports {
		out.Imports[file] = append([]string(nil), imports...)
	}
	out.Excluded = StringSet{}
	for file, exprs := range def.Excluded {
		out.Excluded[file] = append([]string(nil), exprs...)
	}
	out.Types = append(DeclarationList(nil), def.Types...)
	out.Consts = append(DeclarationList(nil), def.Consts...)
	out.Vars = append(DeclarationList(nil), def.Vars...)
	out.Funcs = append(DeclarationList(nil), def.Funcs...)
	return &out
}
