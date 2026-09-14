package docs

import (
	"fmt"
	"io"

	"github.com/titpetric/tools/splint/model"
)

// renderSpec writes the declared symbols alone: one block per package, the
// kind and the names of everything it exports. Verbose keeps the unexported
// symbols in.
func renderSpec(w io.Writer, opts Options, defs model.DefinitionList) error {
	for _, def := range defs {
		if skipDocs(def) {
			continue
		}

		var (
			types  = def.Types.Exported()
			consts = def.Consts.Exported()
			vars   = def.Vars.Exported()
			funcs  = def.Funcs.Exported()
		)

		if opts.Verbose {
			types = def.Types
			consts = def.Consts
			vars = def.Vars
			funcs = def.Funcs
		}

		fmt.Fprintln(w, "package:", packageTitle(def.Package))
		fmt.Fprintln(w, "import:", def.Package.ImportPath)
		fmt.Fprintln(w, "symbols:")

		for _, v := range types {
			fmt.Fprintln(w, "- type:", v.GetNames())
		}
		for _, v := range consts {
			fmt.Fprintln(w, "- const:", v.GetNames())
		}
		for _, v := range vars {
			fmt.Fprintln(w, "- var:", v.GetNames())
		}
		for _, fn := range funcs {
			fmt.Fprintf(w, "- func: `%s`\n", symbol(fn))
		}
	}

	return nil
}
