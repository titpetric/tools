package main

import (
	"go/ast"

	"golang.org/x/tools/go/packages"
)

// skipGeneratedFiles drops generated files from the loaded packages, so no
// analyzer sees them. A file is generated when it carries the conventional
// "// Code generated ... DO NOT EDIT." line, which ast.IsGenerated detects;
// templ, protoc and go:generate tools all emit it. Generated files are not
// hand-organized, so grouping, pairing and coverage expectations do not apply
// to them.
func skipGeneratedFiles(pkgs []*packages.Package) {
	for _, pkg := range pkgs {
		if pkg == nil || len(pkg.Syntax) == 0 {
			continue
		}
		keep := make(map[string]bool, len(pkg.Syntax))
		syntax := pkg.Syntax[:0]
		for _, file := range pkg.Syntax {
			if ast.IsGenerated(file) {
				continue
			}
			syntax = append(syntax, file)
			keep[pkg.Fset.Position(file.Package).Filename] = true
		}
		pkg.Syntax = syntax
		pkg.GoFiles = keepFiles(pkg.GoFiles, keep)
		pkg.CompiledGoFiles = keepFiles(pkg.CompiledGoFiles, keep)
	}
}

// keepFiles filters a file list down to the kept names.
func keepFiles(files []string, keep map[string]bool) []string {
	kept := files[:0]
	for _, file := range files {
		if keep[file] {
			kept = append(kept, file)
		}
	}
	return kept
}
