package docs

import (
	"fmt"
	"io"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// renderMarkdown writes the reference as one markdown document: every
// importable package in turn, each followed by the examples written for it.
func renderMarkdown(w io.Writer, defs model.DefinitionList) error {
	examples := collectExamples(defs)

	for _, def := range defs {
		if skipDocs(def) {
			continue
		}

		if _, err := io.WriteString(w, packageMarkdown(def, examples[def.Package.ImportPath])); err != nil {
			return err
		}
	}

	return nil
}

// packageMarkdown renders one package: the heading, the import, the package
// godoc, the declarations each behind a details fold, the function list with
// a section per function, and the examples.
func packageMarkdown(def *model.Definition, examples model.DeclarationList) string {
	var buf strings.Builder

	var (
		types  = def.Types.Exported()
		consts = def.Consts.Exported()
		vars   = def.Vars.Exported()
		funcs  = def.Funcs.Exported()
	)

	fmt.Fprintf(&buf, "# Package %s\n\n", packageTitle(def.Package))
	fmt.Fprintf(&buf, "```go\n")
	fmt.Fprintf(&buf, "import (\n\t%q\n)\n", def.Package.ImportPath)
	fmt.Fprintf(&buf, "```\n\n")

	if def.Doc != "" {
		fmt.Fprintf(&buf, "%s\n\n", fenceCodeBlocks(citePackage(strings.TrimSpace(def.Doc), def.Package.Package)))
	}

	sections := []struct {
		title string
		decls model.DeclarationList
	}{
		{"Types", types},
		{"Consts", consts},
		{"Vars", vars},
	}
	for _, section := range sections {
		if len(section.decls) == 0 {
			continue
		}

		fmt.Fprintf(&buf, "## %s\n\n", section.title)
		for _, decl := range section.decls {
			if src := declaration(decl); src != "" {
				fmt.Fprint(&buf, details(decl, src))
			}
		}
	}

	if len(funcs) > 0 {
		fmt.Fprint(&buf, "## Function symbols\n\n")

		for _, fn := range funcs {
			fmt.Fprintf(&buf, "- `%s`\n", symbol(fn))
		}
		fmt.Fprint(&buf, "\n")

		// Documented functions first.
		for _, fn := range funcs {
			if fn.Doc == "" {
				continue
			}

			fmt.Fprintf(&buf, "### `%s`\n\n", fn.Name)
			fmt.Fprintf(&buf, "%s\n\n", fenceCodeBlocks(citeName(strings.TrimSpace(fn.Doc), fn.Name)))
			fmt.Fprintf(&buf, "```go\n%s\n```\n\n", symbol(fn))
		}

		// List undocumented ones.
		for _, fn := range funcs {
			if fn.Doc != "" {
				continue
			}

			fmt.Fprintf(&buf, "### `%s`\n\n", fn.Name)
			fmt.Fprintf(&buf, "```go\n%s\n```\n\n", symbol(fn))
		}
	}

	fmt.Fprint(&buf, renderExamples(examples))

	return buf.String()
}

// citePackage writes the name in a "Package name ..." godoc opening as code,
// the way the page cites every other symbol, so a prose linter reads the
// identifier as an identifier.
func citePackage(doc, name string) string {
	if !strings.HasPrefix(doc, "Package ") {
		return doc
	}
	return "Package " + citeName(doc[len("Package "):], name)
}

// citeName writes the name a godoc comment opens with as code, so "Load
// reads" becomes "`Load` reads". It returns any other comment unchanged.
func citeName(doc, name string) string {
	if name == "" || !strings.HasPrefix(doc, name) {
		return doc
	}
	rest := doc[len(name):]
	if rest != "" && !strings.ContainsAny(rest[:1], " \n.,;:") {
		return doc
	}
	return "`" + name + "`" + rest
}
