package visibility

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// MaxInternalRatio is the share of package code the internal funcs may occupy,
// as a percentage, and the one threshold a package is judged on. A quarter of a
// package being private plumbing passes; more than that is what the warning is
// for. The counts are in the row either way: a package with six internal funcs
// holding a tenth of its code is not carrying too much.
const MaxInternalRatio = 25.0

// Analyzer counts the exported and the internal half of every package.
type Analyzer struct{}

// New creates a new visibility analyzer.
func New() *Analyzer {
	return &Analyzer{}
}

// Analyze examines packages and returns their visibility counts.
//
// A symbol is exported or internal by the case of its own name, so a method
// counts as a func and (*Tracer).serveHTTP counts as internal the way a free
// function does. Test files and their packages are left out: a package is
// measured by what it ships.
func (a *Analyzer) Analyze(pkgs []*packages.Package) (*Report, error) {
	reports := make(map[string]*PackageReport)
	seen := make(map[string]bool)

	for _, pkg := range pkgs {
		if skipPackage(pkg) {
			continue
		}
		report := reports[pkg.PkgPath]
		if report == nil {
			report = &PackageReport{Package: packageLabel(pkg)}
			reports[pkg.PkgPath] = report
		}

		for _, file := range pkg.Syntax {
			name := pkg.Fset.Position(file.Pos()).Filename
			if strings.HasSuffix(name, "_test.go") || seen[name] {
				continue
			}
			seen[name] = true
			countFile(report, pkg.Fset, file, blankLines(name))
		}
	}

	out := &Report{Packages: make([]PackageReport, 0, len(reports))}
	for _, report := range reports {
		if report.Lines > 0 {
			report.InternalRatio = float64(report.InternalLines) / float64(report.Lines) * 100
		}
		report.Warnings = warningsFor(*report)
		out.Packages = append(out.Packages, *report)

		out.Total++
		if report.OK() {
			out.Passing++
			continue
		}
		out.Warnings++
	}
	sort.Slice(out.Packages, func(i, j int) bool { return out.Packages[i].Package < out.Packages[j].Package })
	return out, nil
}

// countFile adds the declarations and the code of one file to the package it
// belongs to. The blank set is the lines of the file holding no code, which a
// position range cannot tell on its own.
func countFile(report *PackageReport, fset *token.FileSet, file *ast.File, blank map[int]bool) {
	report.Lines += codeLines(fset, file, blank, file.Pos(), file.End())

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if ast.IsExported(d.Name.Name) {
				report.ExportedFuncs++
				continue
			}
			report.InternalFuncs++
			if d.Body != nil {
				report.InternalLines += codeLines(fset, file, blank, d.Body.Pos(), d.Body.End())
			}
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			for _, spec := range d.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if ast.IsExported(typeSpec.Name.Name) {
					report.ExportedTypes++
					continue
				}
				report.InternalTypes++
			}
		}
	}
}

// codeLines counts the lines of a range that carry code, so blank lines and
// the comments above and inside a declaration are left out.
func codeLines(fset *token.FileSet, file *ast.File, blank map[int]bool, from, to token.Pos) int {
	comments := make(map[int]bool)
	for _, group := range file.Comments {
		if group.Pos() > to || group.End() < from {
			continue
		}
		start := fset.Position(group.Pos()).Line
		end := fset.Position(group.End()).Line
		for line := start; line <= end; line++ {
			comments[line] = true
		}
	}

	lines := 0
	start := fset.Position(from).Line
	end := fset.Position(to).Line
	for line := start; line <= end; line++ {
		if !comments[line] && !blank[line] {
			lines++
		}
	}
	return lines
}

// blankLines returns the lines of a file that hold nothing but whitespace. A
// file that cannot be read reports none, so its code counts in full.
func blankLines(name string) map[int]bool {
	blank := make(map[int]bool)
	data, err := os.ReadFile(name)
	if err != nil {
		return blank
	}
	for i, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			blank[i+1] = true
		}
	}
	return blank
}

// warningsFor returns the thresholds a package crossed.
func warningsFor(report PackageReport) []string {
	if report.InternalRatio > MaxInternalRatio {
		return []string{fmt.Sprintf("%.1f%% internal code, over %.0f%%", report.InternalRatio, MaxInternalRatio)}
	}
	return nil
}

// skipPackage reports whether a package is one the count would distort:
// commands, the synthetic test packages, anything loaded without syntax, and
// the internal tree, which is scoped to the module whatever it exports.
func skipPackage(pkg *packages.Package) bool {
	return pkg.Name == "main" ||
		strings.HasSuffix(pkg.Name, "_test") ||
		strings.HasSuffix(pkg.PkgPath, ".test") ||
		internalPath(pkg.PkgPath) ||
		len(pkg.Syntax) == 0
}

// internalPath reports whether a package sits in an internal tree, which Go
// closes to importers outside the module that declares it.
func internalPath(pkgPath string) bool {
	for _, element := range strings.Split(pkgPath, "/") {
		if element == "internal" {
			return true
		}
	}
	return false
}

// packageLabel is the package path relative to its module, with the module
// root reported as "(root)" because its path says nothing on its own.
func packageLabel(pkg *packages.Package) string {
	if pkg.Module == nil || pkg.Module.Path == "" {
		return pkg.PkgPath
	}
	if pkg.PkgPath == pkg.Module.Path {
		return "(root)"
	}
	if rel := strings.TrimPrefix(pkg.PkgPath, pkg.Module.Path+"/"); rel != pkg.PkgPath {
		return rel
	}
	return path.Base(pkg.PkgPath)
}
