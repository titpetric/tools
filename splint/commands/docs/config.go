package docs

import (
	"fmt"
	"io"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// renderConfig writes one package's types as a configuration reference: each
// type under its own heading, each field as a bold line with its json name
// and its type, and the field doc as the paragraph under it. A field whose
// type the package declares links to that type's section, which is how a
// reader walks a nested config from the root.
//
// The root type prints first and the types it reaches follow in reach
// order, so the page reads from the top of the config down; --title
// replaces the root type's heading.
func renderConfig(w io.Writer, opts Options, defs model.DefinitionList) error {
	def := configPackage(defs, opts.Root)
	if def == nil {
		if opts.Root != "" {
			return fmt.Errorf("no package declares the root type %q", opts.Root)
		}
		return fmt.Errorf("the document holds no package to render")
	}

	types := def.Types
	declared := make(map[string]*model.Declaration, len(types))
	for _, decl := range types {
		for _, name := range decl.GetNames() {
			declared[name] = decl
		}
	}

	for _, decl := range orderFromRoot(types, declared, opts.Root) {
		renderConfigType(w, opts, decl, declared)
	}

	return nil
}

// configPackage returns the package the reference is rendered from: the one
// declaring the root type, or the first importable package when no root is
// named.
func configPackage(defs model.DefinitionList, root string) *model.Definition {
	for _, def := range defs {
		if skipDocs(def) {
			continue
		}
		if root == "" {
			return def
		}
		for _, decl := range def.Types {
			if decl.HasName(root) {
				return def
			}
		}
	}
	return nil
}

// orderFromRoot returns the types with the root first and everything it
// reaches after it, breadth first, then whatever is left in declared order.
// With no root, or one the package does not declare, the declared order
// stands.
func orderFromRoot(types model.DeclarationList, declared map[string]*model.Declaration, root string) []*model.Declaration {
	if _, ok := declared[root]; root == "" || !ok {
		return types
	}

	seen := map[*model.Declaration]bool{}
	queue := []*model.Declaration{declared[root]}
	seen[declared[root]] = true

	var out []*model.Declaration
	for len(queue) > 0 {
		decl := queue[0]
		queue = queue[1:]
		out = append(out, decl)

		for _, name := range referencedTypes(decl) {
			if next, ok := declared[name]; ok && !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}

	for _, decl := range types {
		if !seen[decl] {
			out = append(out, decl)
		}
	}

	return out
}

// referencedTypes returns the declared type names one type reaches: the type
// it is defined as, and the type of each of its fields.
func referencedTypes(decl *model.Declaration) []string {
	var out []string
	if ref := decl.TypeRef(); ref != "" {
		out = append(out, ref)
	}
	for _, field := range decl.Fields {
		if ref := field.TypeRef(); ref != "" {
			out = append(out, ref)
		}
	}
	return out
}

// renderConfigType writes one type: its heading, its doc, and its fields.
// The root type's heading is the title when one was given.
func renderConfigType(w io.Writer, opts Options, decl *model.Declaration, declared map[string]*model.Declaration) {
	if decl.Name == opts.Root && opts.Title != "" {
		fmt.Fprint(w, opts.Title+"\n\n")
	} else {
		fmt.Fprintf(w, "# %s\n\n", decl.Name)
	}

	if doc := strings.TrimSpace(decl.Doc); doc != "" {
		fmt.Fprintf(w, "%s\n\n", fenceCodeBlocks(doc))
	}

	for _, field := range decl.Fields {
		renderConfigField(w, field, declared)
	}

	if len(decl.Fields) > 0 {
		return
	}

	// A typedef has no fields of its own: what it is defined as is the whole
	// of it, and a definition over a declared type links there.
	target := decl.TypeRef()
	if _, ok := declared[target]; !ok || target == "" {
		return
	}

	typeFormat := "Type defined as `%s`"
	shown := decl.Type
	switch {
	case strings.HasPrefix(decl.Type, "[]"):
		typeFormat = "Type defined as array of `%s` values"
		shown = target
	case strings.HasPrefix(decl.Type, "map["):
		typeFormat = "Type defined as object of `%s` values"
		shown = target
	}
	fmt.Fprintf(w, typeFormat+", see [%s](#%s) definition.\n\n", shown, target, strings.ToLower(target))
}

// renderConfigField writes one field: its json name, its type linked to the
// section that declares it where one does, and its doc.
func renderConfigField(w io.Writer, field *model.Field, declared map[string]*model.Declaration) {
	name := field.Name
	if jsonName, _, _ := strings.Cut(field.JSONName, ","); jsonName != "" {
		name = jsonName
	}

	target := field.TypeRef()
	if _, ok := declared[target]; ok && target != "" {
		// A pointer dereferences to the type the section describes, so the
		// star says nothing to a config reader; a slice stays spelled out.
		shown := strings.TrimPrefix(field.Type, "*")
		fmt.Fprintf(w, "**Field: `%s` ([%s](#%s))**\n", name, shown, strings.ToLower(target))
	} else {
		fmt.Fprintf(w, "**Field: `%s` (`%s`)**\n", name, jsonType(field.Type))
	}

	fmt.Fprintf(w, "%s\n\n", strings.TrimSpace(field.Doc))
}

// jsonType names a Go type the way a config reader reads it. A function is
// not something a document carries, so its signature says nothing here.
func jsonType(name string) string {
	switch name {
	case "bool":
		return "boolean"
	case "interface{}", "any", "map[string]interface{}", "map[string]any":
		return "any"
	}
	if strings.HasPrefix(name, "func") {
		return "func"
	}
	return name
}
