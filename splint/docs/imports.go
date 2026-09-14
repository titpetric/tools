package docs

import (
	"fmt"
	"io"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// renderImports writes a plantuml diagram of which packages of the tree
// import which: one arrow per import that resolves to a package the document
// holds, so a dependency outside the tree draws nothing.
func renderImports(w io.Writer, defs model.DefinitionList) error {
	pkgs := make(map[string]model.Package)
	for _, def := range defs {
		pkgs[def.Package.ImportPath] = def.Package
	}

	fmt.Fprintln(w, "@startuml")

	imports := model.NewStringSet()

	for _, def := range defs {
		importMap, _ := def.Imports.Map(def.Imports.All())

		for _, long := range importMap {
			if !strings.Contains(long, ".") {
				continue
			}

			pkg, ok := pkgs[long]
			if !ok {
				continue
			}

			addImport(&imports, def.Package, pkg)
		}
	}

	for _, src := range imports.Keys() {
		for _, use := range imports[src] {
			fmt.Fprintf(w, "[%s] --|> [%s]\n", src, use)
		}
	}

	fmt.Fprintln(w, "@enduml")

	return nil
}

// addImport records one arrow, from the importing package to the imported
// one, under names the diagram can draw.
func addImport(s *model.StringSet, src, dest model.Package) {
	from, to := sanitize(src.ImportPath), sanitize(dest.ImportPath)
	if from == "" {
		from = src.Package
	}
	if to == "" {
		to = dest.Package
	}
	s.Add(from, to)
}

// sanitize takes the relative path decoration off an import path, which is
// what a package of the parse root is recorded under.
func sanitize(n string) string {
	return strings.Trim(n, "./")
}
