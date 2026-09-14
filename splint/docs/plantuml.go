package docs

import (
	"fmt"
	"io"
	"maps"
	"reflect"
	"slices"
	"strings"
	"unicode"

	"github.com/titpetric/tools/splint/model"
)

// dbRelationship describes a database relationship read off a foreign key
// field.
type dbRelationship struct {
	fromType   string
	fromField  string
	toType     string
	isOneToOne bool
	isExternal bool // the target type is not declared in the tree
}

// detectDBRelationships scans a type for database relationships.
//
// It looks for fields with an "ID" suffix and checks whether the target type
// has an ID field of its own. A target with an ID is a 1:N relationship; one
// without makes the foreign key the only key, which is 1:1. A foreign key no
// declared type answers to is drawn as a conceptual type.
func detectDBRelationships(t *model.Declaration, allTypes map[string]*model.Declaration) []dbRelationship {
	var relations []dbRelationship

	for _, f := range t.Fields {
		// A foreign key field is named for what it points at: UserID, PostId
		// or post_id.
		var baseName string
		fieldNameLower := strings.ToLower(f.Name)

		if strings.HasSuffix(fieldNameLower, "id") {
			if len(f.Name) > 2 && (f.Name[len(f.Name)-2:] == "ID" || f.Name[len(f.Name)-2:] == "Id") {
				baseName = f.Name[:len(f.Name)-2]
			} else if strings.HasSuffix(fieldNameLower, "_id") {
				baseName = strings.TrimSuffix(fieldNameLower, "_id")
			} else {
				continue
			}
		} else {
			continue
		}

		// The db tag names the column, and the column names the target more
		// reliably than the field does.
		var targetTypeName string
		var exists bool

		if f.Tag != "" {
			dbCol := dbTagName(f.Tag)
			if dbCol != "" && strings.HasSuffix(strings.ToLower(dbCol), "_id") {
				baseCol := strings.TrimSuffix(strings.ToLower(dbCol), "_id")
				targetTypeName, exists = tryTypeMatches(baseCol, allTypes)
			}
		}

		if targetTypeName == "" {
			targetTypeName, exists = tryTypeMatches(baseName, allTypes)
		}

		// A target the tree does not declare is drawn as a conceptual type,
		// which handles OwnerID -> Owner where no Owner exists.
		isExternal := false
		if !exists {
			targetTypeName = toPascalCase(baseName)
			isExternal = true
		}

		// A source with its own ID can repeat the foreign key across rows,
		// which is 1:N. One without has the foreign key as its only key,
		// which is 1:1.
		relations = append(relations, dbRelationship{
			fromType:   t.Name,
			fromField:  f.Name,
			toType:     targetTypeName,
			isOneToOne: !hasIDField(t),
			isExternal: isExternal,
		})
	}

	return relations
}

// hasIDField reports a type with an "ID" or "Id" field, which is its primary
// key.
func hasIDField(t *model.Declaration) bool {
	for _, f := range t.Fields {
		if f.Name == "ID" || f.Name == "Id" {
			return true
		}
	}
	return false
}

// toPascalCase converts snake_case to PascalCase: "user_profile" becomes
// "UserProfile".
func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if part != "" {
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}
	return strings.Join(parts, "")
}

// tryTypeMatches finds the type a column base names, trying the full name
// first and then progressively shorter suffixes: "actor_user" is ActorUser
// where one is declared, and User otherwise.
func tryTypeMatches(baseName string, allTypes map[string]*model.Declaration) (string, bool) {
	candidate := toPascalCase(baseName)
	if _, exists := allTypes[candidate]; exists {
		return candidate, true
	}

	parts := strings.Split(baseName, "_")
	for i := 1; i < len(parts); i++ {
		candidate := toPascalCase(strings.Join(parts[i:], "_"))
		if _, exists := allTypes[candidate]; exists {
			return candidate, true
		}
	}

	return "", false
}

// hideSet keys the type names the diagram leaves out.
func hideSet(hide []string) map[string]bool {
	out := make(map[string]bool, len(hide))
	for _, name := range hide {
		out[strings.TrimSpace(name)] = true
	}
	return out
}

// renderPlantUML writes a class diagram of the types: one class per struct,
// its fields, the methods on it, and an arrow for every field, embed and
// method that reaches a type the document declares.
func renderPlantUML(w io.Writer, opts Options, defs model.DefinitionList) error {
	var links []string
	externalTypes := make(map[string]bool)

	addLink := func(link string) {
		links = append(links, link)
	}

	hidden := hideSet(opts.Hide)

	fmt.Fprintln(w, "@startuml")
	fmt.Fprintln(w, "")

	allTypes := make(map[string]*model.Declaration)
	allPackages := make(map[string]*model.Package)
	allFuncs := make(map[string][]*model.Declaration)

	for _, def := range defs {
		allPackages[def.Package.ImportPath] = &def.Package
		for _, t := range def.Types {
			allTypes[t.Name] = t
		}
	}

	for _, def := range defs {
		for _, t := range def.Funcs {
			receiver := t.ReceiverTypeRef()
			if receiver == "" {
				continue
			}

			receiver = def.Package.Namespace(".") + receiver

			allFuncs[receiver] = append(allFuncs[receiver], t)
		}
	}

	for _, def := range defs {
		importMap, _ := def.Imports.Map(def.Imports.All())

		lookup := func(name string) (*model.Package, bool) {
			importpath, ok := importMap[name]
			if !ok {
				return nil, false
			}

			pkg, ok := allPackages[importpath]
			return pkg, ok
		}

		if skipDocs(def) {
			continue
		}

		namespace := def.Package.Namespace(".")

		for _, t := range def.Types {
			if len(t.Fields) == 0 {
				continue
			}

			if hidden[t.Name] {
				continue
			}

			// Model mode draws the data alone.
			if opts.Model && strings.HasPrefix(t.Type, "interface") {
				continue
			}

			dbRelations := detectDBRelationships(t, allTypes)
			for _, rel := range dbRelations {
				relType := "1:N"
				if rel.isOneToOne {
					relType = "1:1"
				}
				if rel.isExternal {
					externalTypes[rel.toType] = true
				}
				addLink(fmt.Sprintf("%q --> %q : .%s (%s)", namespace+t.Name, namespace+rel.toType, rel.fromField, relType))
			}

			for _, name := range t.GetNames() {
				token := "class"
				if strings.HasPrefix(t.Type, "interface") {
					token = "interface"
				}

				for _, f := range t.Fields {
					if strings.HasPrefix(f.Type, "func") {
						token = "interface"
					}
				}

				if len(t.Arguments) > 0 {
					name += "[" + strings.Join(t.Arguments, ", ") + "]"
				}

				fmt.Fprintln(w, token, fmt.Sprintf("%q", namespace+name), "{")
				for _, f := range t.Fields {
					typeRef := f.TypeRef()

					// An embedded field is inheritance rather than a field.
					if f.Name == "" {
						if strings.Contains(typeRef, ".") {
							packageName, typeName, _ := strings.Cut(typeRef, ".")
							if p, ok := lookup(packageName); ok {
								addLink(fmt.Sprintf("%q --|> %q : embeds", namespace+name, p.Namespace(".")+typeName))
							}
						} else {
							addLink(fmt.Sprintf("%q --|> %q : embeds", namespace+name, namespace+typeRef))
						}
						continue
					}

					fieldType := f.Type
					if strings.HasPrefix(fieldType, "struct") {
						fieldType = "struct"
					}
					if strings.HasPrefix(fieldType, "interface") {
						fieldType = "interface"
					}

					marker := "-"
					if exportedName(f.Name) {
						marker = "+"
					}
					fmt.Fprintln(w, " ", marker, f.Name+":", fieldType)

					if token != "interface" {
						if strings.Contains(typeRef, ".") {
							packageName, typeName, _ := strings.Cut(typeRef, ".")
							if p, ok := lookup(packageName); ok {
								addLink(fmt.Sprintf("%q --> %q : .%s", namespace+name, p.Namespace(".")+typeName, f.Name))
							}
						} else if _, ok := model.ToType(typeRef); ok {
							addLink(fmt.Sprintf("%q --> %q : .%s", namespace+name, namespace+typeRef, f.Name))
						}
					}
				}

				addLink("")

				if token == "interface" {
					continue
				}

				if opts.Model {
					continue
				}

				for _, sig := range allFuncs[namespace+t.Name] {
					// A method reaching a declared type in an argument or a
					// return draws one arrow, named for the method.
					methodLink := func() string {
						for _, argType := range append(append([]string{}, sig.Returns...), sig.Arguments...) {
							typeRef := model.TypeRef(argType)

							if !strings.Contains(typeRef, ".") {
								continue
							}
							packageName, typeName, _ := strings.Cut(typeRef, ".")
							if p, ok := lookup(packageName); ok {
								return fmt.Sprintf("%q --> %q : .%s()", namespace+name, p.Namespace(".")+typeName, sig.Name)
							}
						}
						return ""
					}
					if link := methodLink(); link != "" {
						addLink(link)
					}

					if exportedName(sig.Signature) {
						fmt.Fprintln(w, "  +", sig.Signature)
					}
				}
			}

			fmt.Fprintln(w, "}")
			fmt.Fprintln(w)
		}
	}

	// Conceptual types are drawn as interfaces, so a foreign key pointing
	// outside the tree still lands somewhere.
	if len(externalTypes) > 0 {
		fmt.Fprintln(w)
		for _, typeName := range slices.Sorted(maps.Keys(externalTypes)) {
			fmt.Fprintf(w, "interface %q\n", typeName)
		}
	}

	for _, link := range links {
		fmt.Fprintln(w, link)
	}
	if len(links) > 0 {
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "@enduml")

	return nil
}

// exportedName reports a name that opens on an upper case letter, which is
// what makes it visible outside its package.
func exportedName(name string) bool {
	for _, r := range name {
		return unicode.IsUpper(r)
	}
	return false
}

// dbTagName is the column a field maps to, which is the name in its db tag.
func dbTagName(tag string) string {
	value := reflect.StructTag(tag).Get("db")
	name, _, _ := strings.Cut(value, ",")
	return name
}
