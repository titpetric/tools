package model

import (
	"sort"
	"strings"
)

// DeclarationList holds a list of Go symbols.
type DeclarationList []*Declaration

// Append adds declarations to the list.
func (p *DeclarationList) Append(in ...*Declaration) {
	*p = append(*p, in...)
}

// AppendUnique adds the declarations not yet in the list and sorts it.
func (p *DeclarationList) AppendUnique(in ...*Declaration) {
	for _, i := range in {
		shouldAppend := true
		for _, decl := range *p {
			if decl.Equal(i) {
				shouldAppend = false
				break
			}
		}

		if shouldAppend {
			*p = append(*p, i)
		}
	}
	p.Sort()
}

// Exported is the exported part of the list.
func (p DeclarationList) Exported() (result DeclarationList) {
	for _, decl := range p {
		if decl.IsExported() {
			result = append(result, decl)
		}
	}
	return
}

// Walk calls matchfn for each declaration.
func (p DeclarationList) Walk(matchfn func(d *Declaration)) {
	for _, decl := range p {
		matchfn(decl)
	}
}

// Find returns the first declaration matchfn accepts,
// or nil.
func (p DeclarationList) Find(matchfn func(d *Declaration) bool) *Declaration {
	for _, decl := range p {
		if matchfn(decl) {
			return decl
		}
	}
	return nil
}

// Filter is the declarations matchfn accepts.
func (p DeclarationList) Filter(matchfn func(d *Declaration) bool) (result DeclarationList) {
	for _, decl := range p {
		if matchfn(decl) {
			result.Append(decl)
		}
	}
	return
}

// ClearSource drops the source of every declaration.
func (p *DeclarationList) ClearSource() {
	for _, decl := range *p {
		decl.Source = ""
	}
}

// ClearTestFiles drops the declarations of _test.go files.
func (p *DeclarationList) ClearTestFiles() {
	result := DeclarationList{}
	for _, decl := range *p {
		if strings.HasSuffix(decl.File, "_test.go") {
			continue
		}
		result.Append(decl)
	}
	*p = result
}

// ClearNonTestFiles keeps only the declarations of _test.go files.
func (p *DeclarationList) ClearNonTestFiles() {
	result := DeclarationList{}
	for _, decl := range *p {
		if !strings.HasSuffix(decl.File, "_test.go") {
			continue
		}
		result.Append(decl)
	}
	*p = result
}

// Sort orders by kind, exported first, then receiver, signature and name.
func (p *DeclarationList) Sort() {
	sort.Slice(*p, func(i, j int) bool {
		a, b := (*p)[i], (*p)[j]
		if a.Kind != b.Kind {
			indexOf := map[DeclarationKind]int{
				CommentKind: 0,
				ImportKind:  1,
				ConstKind:   2,
				StructKind:  3,
				TypeKind:    4,
				VarKind:     5,
				FuncKind:    6,
			}
			return indexOf[a.Kind] < indexOf[b.Kind]
		}
		ae, be := isExported(a.Name), isExported(b.Name)
		if ae != be {
			return ae
		}

		if a.Receiver != b.Receiver {
			if a.Receiver == "" {
				return true
			}
			return a.Receiver < b.Receiver
		}

		if a.Signature != b.Signature {
			return a.Signature < b.Signature
		}

		return a.Name < b.Name
	})
}
