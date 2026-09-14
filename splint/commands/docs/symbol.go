package docs

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/refindex"
)

// renderSymbol prints one symbol the way go doc would, and then what the
// document knows that godoc does not: how many tests reach it.
//
// The query is matched against every package of the document, "Open" against
// names and "Client.Close" against methods, and every match is printed: a
// name repeating across packages is a fact of the tree, not a tie to break.
// Verbose adds one line per reference, tests and product code both, at the
// position of the declaration doing the referencing: the model records who
// references a symbol, not each place they do it from.
func renderSymbol(w io.Writer, root *model.DocumentRoot, opts Options) error {
	index := refindex.Build(root)

	found := 0
	for _, def := range root.Packages {
		if skipDocs(def) {
			continue
		}

		for _, decl := range def.Order() {
			if decl.IsTestScope() {
				continue
			}
			if decl.Symbol() != opts.Symbol && !decl.HasName(opts.Symbol) {
				continue
			}

			if found > 0 {
				fmt.Fprintln(w)
			}
			found++

			if err := writeSymbol(w, index, def, decl, opts.Verbose); err != nil {
				return err
			}
		}
	}

	if found == 0 {
		return fmt.Errorf("no symbol %q in the loaded packages", opts.Symbol)
	}
	return nil
}

// writeSymbol prints one declaration: the package it is in, the declaration
// itself, its godoc, and the tests that reach it.
func writeSymbol(w io.Writer, index *refindex.Index, def *model.Definition, decl *model.Declaration, verbose bool) error {
	fmt.Fprintf(w, "package %s // import %q\n\n", def.Package.Package, def.Package.ImportPath)

	if decl.Kind == model.FuncKind {
		fmt.Fprintln(w, symbol(decl))
		if doc := strings.TrimSpace(decl.Doc); doc != "" {
			fmt.Fprintln(w)
			for _, line := range strings.Split(doc, "\n") {
				fmt.Fprintln(w, strings.TrimRight("    "+line, " "))
			}
		}
	} else {
		fmt.Fprintln(w, strings.TrimSpace(declaration(decl)))
	}

	key := refindex.Key{ImportPath: def.Package.ImportPath, Symbol: decl.Symbol()}

	fmt.Fprintf(w, "\nTested by %s.\n", plural(len(index.TestRefs(key)), "test"))

	if !verbose {
		return nil
	}

	refs := index.Refs(key)
	if len(refs) == 0 {
		return nil
	}

	sort.SliceStable(refs, func(i, j int) bool {
		a, b := refs[i].Decl.Position(refs[i].Pkg), refs[j].Decl.Position(refs[j].Pkg)
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})

	fmt.Fprintf(w, "\nReferences:\n")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, ref := range refs {
		kind := "product"
		if ref.Test {
			kind = "test"
		}
		fmt.Fprintf(tw, "  %s\t%s.%s\t%s\n", kind, ref.Pkg.Package, ref.Decl.Symbol(), position(ref))
	}
	return tw.Flush()
}

// position is where a referencing declaration is, as a path a reader opens.
func position(ref refindex.Ref) string {
	return ref.Decl.Position(ref.Pkg).Ref()
}

// plural counts a thing in a sentence, so "1 test" reads as a sentence.
func plural(count int, word string) string {
	if count == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", count, word)
}
