// Package testfacts reads what the integration rule needs from the test
// declarations of a package, the same way for both parsers.
//
// The facts come off the source every declaration carries before the parse
// trims it, so the two parsers pass the same text to the same scanner and
// cannot disagree about what a literal holds.
package testfacts

import (
	"go/scanner"
	"go/token"
	"sort"
	"strconv"

	"github.com/titpetric/tools/splint/model"
)

// Fill records the string literals of every declaration in a test file of
// the definition. A parser calls it before it clears the sources.
func Fill(def *model.Definition) {
	for _, decl := range def.Order() {
		if !decl.IsTestScope() || decl.Source == "" {
			continue
		}
		decl.Literals = literals(decl.Source)
	}
}

// literals returns the string literals the source writes, unquoted, sorted and
// deduplicated. A raw string and an interpreted one read the same, and an
// import path is no literal of a declaration, because the source of a
// declaration holds no import.
func literals(source string) []string {
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(source))

	var s scanner.Scanner
	// The source is a declaration out of context, so an error means a
	// fragment the scanner did not expect, and the scan goes on past it.
	s.Init(file, []byte(source), nil, 0)

	seen := map[string]bool{}
	var out []string
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok != token.STRING {
			continue
		}
		value, err := strconv.Unquote(lit)
		if err != nil || value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}

	sort.Strings(out)
	return out
}
