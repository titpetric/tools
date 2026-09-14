// Package coverreport renders the coverage a document carries as tables.
//
// The coverage itself is folded in by coverprofile when the document is
// parsed. What is rendered here is the report over it: one row per function
// with the coverage and the cognitive complexity beside it, and one row per
// package with the lines added up. A template turns the two tables into a
// page; without one the function table alone is printed.
package coverreport

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/template"

	"github.com/titpetric/tools/splint/model"
	"github.com/titpetric/tools/splint/render"
)

// The row markers. A function passes when it is trivial or covered: no
// measured complexity, coverage above 80%, or some coverage on a function of
// cognitive complexity five or less.
const (
	markPass = "✅"
	markFail = "❌"
)

// Options is what one report was asked for.
type Options struct {
	// Template names a text/template file rendered with .Functions and
	// .Packages, the two tables as markdown. Without one, the function table
	// alone is printed.
	Template string

	// Verbose keeps the functions with no coverage in the report, which are
	// left out otherwise.
	Verbose bool
}

// Row is one function of the report.
type Row struct {
	Package   string
	Function  string
	Coverage  float64
	Cognitive int
}

// Data returns the function rows of the report, sorted by package and
// function. Without verbose, a function with no coverage is left out.
func Data(root *model.DocumentRoot, verbose bool) []Row {
	var result []Row

	for _, def := range root.Packages {
		fns := def.Funcs.Filter(func(d *model.Declaration) bool {
			if verbose {
				return true
			}
			return d.Complexity != nil && d.Complexity.Coverage > 0
		})
		for _, fn := range fns {
			row := Row{
				Package:  def.Package.ImportPath,
				Function: combined(fn.Receiver, fn.Name),
			}
			if fn.Complexity != nil {
				row.Coverage = fn.Complexity.Coverage
				row.Cognitive = fn.Complexity.Cognitive
			}
			result = append(result, row)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		k, v := result[i], result[j]
		if k.Package != v.Package {
			return k.Package < v.Package
		}
		if k.Function != v.Function {
			return k.Function < v.Function
		}
		return k.Coverage > v.Coverage
	})

	return result
}

// Write renders the report to w: the template with both tables when one is
// named, and the function table alone otherwise.
func Write(w io.Writer, root *model.DocumentRoot, opts Options) error {
	module := modulePath(root.Packages)

	functions := functionTable(Data(root, opts.Verbose), module)
	packages := packageTable(root.Packages, module)

	if opts.Template == "" {
		_, err := fmt.Fprintln(w, functions)
		return err
	}

	tmpl, err := template.ParseFiles(opts.Template)
	if err != nil {
		return err
	}

	data := struct {
		Packages  string
		Functions string
	}{
		Packages:  packages,
		Functions: functions,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	_, err = fmt.Fprintln(w, buf.String())
	return err
}

// functionTable is the per function table: a pass mark, the package, the
// function, and the coverage and cognitive complexity behind the mark.
func functionTable(rows []Row, module string) string {
	cells := make([][]string, 0, len(rows))
	for _, r := range rows {
		cells = append(cells, []string{
			mark(r.Coverage, r.Cognitive),
			relativePackage(module, r.Package),
			r.Function,
			fmt.Sprintf("%.2f%%", r.Coverage),
			fmt.Sprint(r.Cognitive),
		})
	}
	collapseColumn(cells, 1)

	return strings.TrimSpace(render.MarkdownTable([]string{"Status", "Package", "Function", "Coverage", "Cognitive"}, cells))
}

// packageTable is the per package table, read off model.Package.Complexity,
// which is what coverprofile fills when the profile is folded in.
func packageTable(defs model.DefinitionList, module string) string {
	packages := make([]model.Package, 0, len(defs))
	for _, def := range defs {
		packages = append(packages, def.Package)
	}
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].ImportPath < packages[j].ImportPath
	})

	cells := make([][]string, 0, len(packages))
	for _, pkg := range packages {
		complexity := pkg.Complexity
		if complexity == nil {
			complexity = &model.Complexity{}
		}

		cells = append(cells, []string{
			mark(complexity.Coverage, complexity.Cognitive),
			relativePackage(module, pkg.ImportPath),
			fmt.Sprintf("%.2f%%", complexity.Coverage),
			fmt.Sprint(complexity.Cognitive),
			fmt.Sprint(complexity.Lines),
		})
	}

	return strings.TrimSpace(render.MarkdownTable([]string{"Status", "Package", "Coverage", "Cognitive", "Lines"}, cells))
}

// mark is the pass or fail marker of one row.
func mark(coverage float64, cognitive int) string {
	if cognitive == 0 || coverage > 80 || (coverage > 0 && cognitive <= 5) {
		return markPass
	}
	return markFail
}

// combined names a method by its receiver and a function by itself.
func combined(receiver, name string) string {
	if receiver != "" {
		return strings.TrimLeft(receiver, "*") + "." + name
	}
	return name
}

// modulePath returns the module every definition belongs to, or an empty
// string when the tree holds no go.mod or spans more than one module. The
// report names packages relative to it, and relative to what is a question a
// tree of two modules has two answers to.
func modulePath(defs model.DefinitionList) string {
	var path string
	for _, def := range defs {
		if def.Module == nil || def.Module.Path == "" {
			continue
		}
		if path != "" && path != def.Module.Path {
			return ""
		}
		path = def.Module.Path
	}
	return path
}

// relativePackage renames an import path to what it is called inside its own
// module. A report of one module repeats the module path on every row, where
// what tells two rows apart is what follows it; the root package has nothing
// following it and is named ".", as a path relative to the module root is.
func relativePackage(module, importPath string) string {
	if module == "" {
		return importPath
	}
	if importPath == module {
		return "."
	}
	if rest, ok := strings.CutPrefix(importPath, module+"/"); ok {
		return rest
	}
	return importPath
}

// collapseColumn blanks a cell that repeats the row above it, so a column
// reads as the groups it holds rather than as the same value written out
// once per row. The rows are already sorted by that column, so a value
// reappearing after a different one is a different group and is written
// again.
func collapseColumn(rows [][]string, column int) {
	previous := ""
	for _, row := range rows {
		if column >= len(row) {
			continue
		}
		current := row[column]
		if current == previous {
			row[column] = ""
			continue
		}
		previous = current
	}
}
