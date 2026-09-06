package imports

import (
	"fmt"
	"sort"
	"strings"

	"github.com/titpetric/tools/splint/gomod"
	"github.com/titpetric/tools/splint/importfmt"
	"github.com/titpetric/tools/splint/model"
)

// Spread is how far one import path has reached into the tree.
//
// One file is the clean reading. A dependency reached from one file through
// one name is one place to change when it is replaced, and one place a reader
// has to understand to know what the tree took on.
type Spread struct {
	// Path is the import path.
	Path string

	// Files is how many files import it, Packages how many packages those
	// files belong to, and Worst the most files of any one package.
	Files    int
	Packages int
	Worst    int

	// WorstPackage is the package Worst was counted in.
	WorstPackage string

	// Local reports a package of the tree rather than a dependency of it.
	Local bool

	// Std reports a package of the standard library.
	Std bool

	// packages is what the counts are taken from, kept so a file importing a
	// path twice counts once.
	packages map[string]map[string]bool
}

// External reports a dependency the tree took on, which is the only thing the
// rule judges.
func (s *Spread) External() bool {
	return !s.Local && !s.Std
}

// spread reports the external dependencies that have reached more of the tree
// than one place, and records what every import path reached.
//
// A package of the tree is counted and never reported. A model package
// imported by every consumer under it is what a package structure is for, and
// a rule that called it pollution would be a rule against writing one.
func (l *Linter) spread(root *model.DocumentRoot, results *Results) {
	catalogue := gomod.NewCatalogue(root)

	byPath := map[string]*Spread{}
	files := 0

	for _, def := range root.Packages {
		for _, file := range def.Files {
			if file.Test && !l.Pollution.Tests {
				continue
			}
			files++

			for _, spec := range file.ImportDecls.Specs() {
				entry, known := byPath[spec.Path]
				if !known {
					entry = &Spread{
						Path:     spec.Path,
						Local:    catalogue.Owns(spec.Path),
						Std:      importfmt.Group(model.ImportSpec{Path: spec.Path}, importfmt.Options{}) == importfmt.GroupStd,
						packages: map[string]map[string]bool{},
					}
					byPath[spec.Path] = entry
				}

				// The count is per directory rather than per import path. A
				// package and its test half are two definitions of one
				// directory, and a reader asking how many files of a package
				// reach a dependency counts the tests among them.
				where := packageDir(def)
				if entry.packages[where] == nil {
					entry.packages[where] = map[string]bool{}
				}
				entry.packages[where][file.Name] = true
			}
		}
	}

	for _, entry := range byPath {
		for where, seen := range entry.packages {
			entry.Packages++
			entry.Files += len(seen)
			if len(seen) > entry.Worst {
				entry.Worst, entry.WorstPackage = len(seen), where
			}
		}
	}

	results.spread = sortSpread(byPath)
	results.files = files

	for _, entry := range results.spread {
		if !entry.External() {
			continue
		}

		reason := l.polluted(entry, files)
		if reason == "" {
			continue
		}

		owner := entry.Path
		if required, ok := catalogue.Owner(entry.Path); ok {
			owner = required.Path
		}

		results.add(results.packageMetric(entry.WorstPackage), Result{
			Rule:     RulePollution,
			Severity: model.SeverityWarn,
			Position: model.Position{File: "go.mod"},
			Symbol:   owner,
			Message: fmt.Sprintf("%s is imported from %s in %s: %s", entry.Path,
				plural(entry.Files, "file"), plural(entry.Packages, "package"), reason),
		})
	}
}

// packageDir is the directory a package sits in, as a reader opens it.
func packageDir(def *model.Definition) string {
	dir := strings.TrimPrefix(strings.TrimPrefix(def.Package.Path, "."), "/")
	if dir == "" {
		return "."
	}
	return dir
}

// plural writes a count and the thing it counts.
func plural(n int, thing string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, thing)
	}
	return fmt.Sprintf("%d %ss", n, thing)
}

// polluted is why a dependency is reported, and is empty for one that is not.
func (l *Linter) polluted(entry *Spread, files int) string {
	var reasons []string

	if l.Pollution.PerPackage > 0 && entry.Worst >= l.Pollution.PerPackage {
		reasons = append(reasons, fmt.Sprintf("%d of them in %s alone", entry.Worst, entry.WorstPackage))
	}
	if l.Pollution.FileShare > 0 && files > 0 && float64(entry.Files) >= l.Pollution.FileShare*float64(files) {
		reasons = append(reasons, fmt.Sprintf("%.0f%% of the tree", 100*float64(entry.Files)/float64(files)))
	}

	return strings.Join(reasons, " and ")
}

// sortSpread orders the paths so a table reads the same every run: the widest
// reach first, and the path as the tie break.
func sortSpread(byPath map[string]*Spread) []*Spread {
	out := make([]*Spread, 0, len(byPath))
	for _, entry := range byPath {
		out = append(out, entry)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Files != out[j].Files {
			return out[i].Files > out[j].Files
		}
		return out[i].Path < out[j].Path
	})

	return out
}
