package simpleparser

import (
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// selfContained reports whether a declaration names only types declared where
// it is, which is what makes it something a reader can understand without
// looking anywhere else.
//
// A struct or an interface is self contained when no field reaches another
// package and no field names a type the same file declares. Everything else is
// not: a named type, a func type and a value declaration all say what they are
// somewhere other than here.
func (f *file) selfContained(kind model.DeclarationKind, entries []entry) bool {
	switch kind {
	case model.TypeKind:
		for _, in := range entries {
			if !f.selfContainedType(in) {
				return false
			}
		}
		return true
	case model.ConstKind, model.VarKind:
		for _, in := range entries {
			if f.reachesOtherTypes(valueType(in.code)) {
				return false
			}
		}
		return true
	}
	return false
}

// selfContainedType reports whether one type spec is self contained.
func (f *file) selfContainedType(in entry) bool {
	_, rest, found := strings.Cut(in.code, " ")
	if !found {
		return false
	}

	rest = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest), "{"))
	if !strings.HasPrefix(rest, "struct") && !strings.HasPrefix(rest, "interface") {
		// A named type is defined as something declared elsewhere.
		return false
	}

	for _, field := range in.fieldTypes {
		if f.reachesOtherTypes(field) {
			return false
		}
	}
	return true
}

// valueType is the type a const or var spec declares, and is empty for one
// that takes its type from what it is assigned.
func valueType(code string) string {
	head := code
	if eq := indexTop(head, '='); eq >= 0 {
		head = strings.TrimSpace(head[:eq])
	}

	names := splitTop(head, ',')
	if len(names) == 0 {
		return ""
	}

	last := names[len(names)-1]
	if space := indexTop(last, ' '); space >= 0 {
		return strings.TrimSpace(last[space+1:])
	}
	return ""
}

// reachesOtherTypes reports whether a type expression names something declared
// elsewhere: a package qualified name, or a type this file declares.
//
// A type from another file of the same package is not one of them, which looks
// wrong and is not: go/parser resolves an identifier against the file it is
// written in, so that is the scope the model was built against.
func (f *file) reachesOtherTypes(typ string) bool {
	if typ == "" {
		return false
	}

	for _, word := range identifiers(typ) {
		if word.qualified || f.declared[word.name] {
			return true
		}
	}
	return false
}

// word is one identifier of a type expression, and whether it is reached
// through a package.
type word struct {
	name      string
	qualified bool
}

// identifiers reads the identifiers out of a type expression.
func identifiers(typ string) []word {
	var words []word

	for i := 0; i < len(typ); i++ {
		if !isIdentifierByte(typ[i]) {
			continue
		}
		start := i
		for i < len(typ) && isIdentifierByte(typ[i]) {
			i++
		}
		name := typ[start:i]
		if name[0] >= '0' && name[0] <= '9' {
			continue
		}

		// A name followed by a dot is a package, and the name after the dot is
		// what it reaches.
		if i < len(typ) && typ[i] == '.' {
			words = append(words, word{name: name, qualified: true})
			continue
		}
		if start > 0 && typ[start-1] == '.' {
			continue
		}
		words = append(words, word{name: name})
	}

	return words
}
