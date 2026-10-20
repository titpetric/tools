package diff

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// declKind is the kind the report lists a symbol under. It comes from the list
// of the definition holding the symbol, and the declaration itself plays no
// part: a struct type declaration says "struct" but reads as a type.
const (
	kindType  = "type"
	kindConst = "const"
	kindVar   = "var"
	kindFunc  = "func"
)

// The shapes the report lists a type under. The model records nothing for a
// struct shape, since it describes a struct by its fields.
const (
	underlyingStruct    = "struct"
	underlyingInterface = "interface"
)

// entry is one exported declaration, in the form the report lists and the
// comparison reads.
type entry struct {
	symbol Symbol

	// normalized holds the signature with parameter names removed, the form
	// the comparison matches two revisions of a func on, and the shape of a
	// type declaration, the form it matches two revisions of a type on.
	normalized string

	// fields are the exported fields of a struct, or the methods of an
	// interface, by name. Every other kind leaves it nil.
	fields map[string]Field
}

// symbols returns the declarations of defs, keyed so that the same declaration
// in two revisions of a package produces the same key.
//
// symbols skips test packages and commands. It skips internal packages and
// unexported declarations too unless the include flags hold true: neither
// belongs to the API another module can depend on, and the report lists both
// separately when a caller reviews a refactor in place of a release.
func symbols(defs []*model.Definition, includeInternal, includeUnexported bool) map[string]entry {
	result := make(map[string]entry)
	for _, def := range defs {
		if def.TestPackage || def.Package.Package == "main" {
			continue
		}
		if !includeInternal && isInternal(def.ImportPath) {
			continue
		}

		lists := []struct {
			kind  string
			decls model.DeclarationList
		}{
			{kindType, def.Types},
			{kindConst, def.Consts},
			{kindVar, def.Vars},
			{kindFunc, def.Funcs},
		}
		for _, list := range lists {
			for _, decl := range list.decls {
				for _, name := range declNames(decl, includeUnexported) {
					symbol := Symbol{
						Key:      def.ImportPath + "." + name,
						Package:  def.ImportPath,
						Name:     name,
						Kind:     list.kind,
						Exported: exportedSymbol(def.ImportPath, name),
					}
					var (
						normalized string
						fields     map[string]Field
					)
					switch {
					case list.kind == kindFunc && decl.Signature != "":
						symbol.Signature = signature(decl)
						normalized = normalizeSignature(decl.Signature)
					case list.kind == kindType:
						// A func carries its whole body in Source, which is
						// no use to a reader; a type carries its shape, which
						// is the thing worth printing.
						symbol.Definition = sourceWithoutDoc(decl.Source)
						symbol.Underlying = underlying(decl)
						symbol.Fields = exportedFields(decl)

						// The shape reads as the declaration writes it, so a
						// type that changes shape lands in the report the way a
						// func that changes its signature does.
						normalized = "type " + name + " " + symbol.Underlying
						fields = fieldIndex(symbol.Fields)
					}
					result[symbol.Key] = entry{symbol: symbol, normalized: normalized, fields: fields}
				}
			}
		}
	}
	return result
}

// declNames returns the names of a declaration, each qualified with
// its receiver type.
//
// declNames filters the names one by one and skips Declaration.IsExported,
// which reports on the whole group: an unexported name sharing a "const (...)"
// block with an exported one would otherwise come along with it.
func declNames(decl *model.Declaration, includeUnexported bool) []string {
	// A method is only reachable when its receiver type is.
	if !includeUnexported && decl.Receiver != "" && !ast.IsExported(model.TypeRef(decl.Receiver)) {
		return nil
	}

	var names []string
	for _, name := range decl.GetNames() {
		if !includeUnexported && !ast.IsExported(name) {
			continue
		}
		names = append(names, strings.Trim(decl.Receiver+"."+name, "*."))
	}
	return names
}

// underlying returns the shape a type declaration states: the type the
// definition names, "interface" for an interface, and "struct" for a struct.
//
// The model records no type for a struct, describing it by its fields instead,
// so an empty one reads as a struct. Every other shape names itself.
func underlying(decl *model.Declaration) string {
	if decl.Type == "" {
		return underlyingStruct
	}
	return decl.Type
}

// exportedFields returns the exported fields of a struct, or the methods of an
// interface, in name order. exportedFields skips unexported fields: another
// module cannot reach them, so they are not a promise to keep.
func exportedFields(decl *model.Declaration) []Field {
	var fields []Field
	for _, field := range decl.Fields {
		name, embedded := fieldName(field)
		if !ast.IsExported(name) {
			continue
		}
		fields = append(fields, Field{
			Name:     name,
			Type:     field.Type,
			Tag:      field.Tag,
			Embedded: embedded,
		})
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
	return fields
}

// fieldName returns the name a selector reaches a field by, and whether the
// field embeds a type.
//
// The model records an embedded field with no name of its own, and a selector
// reaches it by the last identifier of the type it embeds: "platform.Module"
// reads as "Module". An embedded exported type is part of the API, since it promotes its
// method set into the one embedding it.
func fieldName(field *model.Field) (name string, embedded bool) {
	if field.Name != "" {
		return field.Name, false
	}

	name = strings.TrimPrefix(field.Type, "*")
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		name = name[dot+1:]
	}
	return name, true
}

// fieldIndex keys a field list by name, the key the comparison matches two
// revisions of a type through.
func fieldIndex(fields []Field) map[string]Field {
	index := make(map[string]Field, len(fields))
	for _, field := range fields {
		index[field.Name] = field
	}
	return index
}

// signature renders a func the way its declaration reads, the form the
// listing shows a reader. The receiver keeps the pointer the source gave it, so
// it reads as the method set does.
func signature(decl *model.Declaration) string {
	if decl.Receiver != "" {
		return "func (" + decl.Receiver + ") " + decl.Signature
	}
	return "func " + decl.Signature
}

// sourceWithoutDoc returns a declaration as the source writes it, with the doc
// comment lines above it removed. It drops only the run of comments the
// declaration opens with, so a comment inside the body stays in place.
//
// The model holds no source at all unless the extraction ran with
// --include-sources, in which case sourceWithoutDoc returns nothing.
func sourceWithoutDoc(source string) string {
	source = strings.TrimSpace(source)
	for strings.HasPrefix(source, "//") {
		_, rest, ok := strings.Cut(source, "\n")
		if !ok {
			return ""
		}
		source = strings.TrimSpace(rest)
	}
	return source
}

// isInternal reports whether an import path holds an internal path segment,
// which keeps the package out of reach of any module but its own.
func isInternal(importPath string) bool {
	for _, segment := range strings.Split(importPath, "/") {
		if segment == "internal" {
			return true
		}
	}
	return false
}

// normalizeSignature drops the parameter names of a signature, so that renaming
// a parameter does not read as an API change while changing its type does.
// The model records signatures as "Name (params) result", where a named parameter
// is "a,b Type" and an unnamed one is the type alone.
//
// Names nested inside a parameter type, as in "fn func(path string) error",
// stay untouched; renaming one of those still reads as a change.
func normalizeSignature(signature string) string {
	open := strings.IndexByte(signature, '(')
	if open < 0 {
		return signature
	}
	end := matchParen(signature, open)
	if end < 0 {
		return signature
	}

	params := splitParams(signature[open+1 : end])
	for i, param := range params {
		params[i] = stripParamName(param)
	}
	return signature[:open+1] + strings.Join(params, ", ") + signature[end:]
}

// matchParen returns the index of the parenthesis closing the one at open, or
// -1 when the signature has no closing match.
func matchParen(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// splitParams splits a parameter list on the ", " separating one parameter from
// the next, leaving the ones nested inside a type such as "func(a, b int)"
// in place. The model joins the names of a single parameter without a space,
// so "a,b int" stays whole.
func splitParams(params string) []string {
	if strings.TrimSpace(params) == "" {
		return nil
	}

	var (
		result []string
		depth  int
		start  int
	)
	for i := 0; i < len(params); i++ {
		switch params[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 && i+1 < len(params) && params[i+1] == ' ' {
				result = append(result, params[start:i])
				start = i + 2
				i++
			}
		}
	}
	return append(result, params[start:])
}

// stripParamName removes the leading name list of a parameter. A parameter
// with no name is its type alone, and stripParamName returns it unchanged.
func stripParamName(param string) string {
	param = strings.TrimSpace(param)
	space := strings.IndexByte(param, ' ')
	if space < 0 || !isNameList(param[:space]) {
		return param
	}
	return param[space+1:]
}

// isNameList reports whether s is the comma separated name list of a parameter,
// as opposed to the start of a type. Keywords are not identifiers, so a type such
// as "chan int" or "struct { A int }" is not mistaken for a named parameter.
func isNameList(s string) bool {
	if s == "" {
		return false
	}
	for _, name := range strings.Split(s, ",") {
		if !token.IsIdentifier(name) {
			return false
		}
	}
	return true
}

// exportedSymbol reports whether a declaration is part of the API: the name
// and every name qualifying it start with an upper case letter, and other
// modules can import the package.
func exportedSymbol(importPath, name string) bool {
	if isInternal(importPath) {
		return false
	}
	for _, part := range strings.Split(name, ".") {
		if !ast.IsExported(part) {
			return false
		}
	}
	return true
}
