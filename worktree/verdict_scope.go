package main

// scopeRoot is the package pattern that narrows a verdict to the package at
// the module root, read the way the go tool reads ".". The implicit pattern
// is "./...", the whole module, which is no narrowing at all.
const scopeRoot = "."

// rootScopeArg reports whether a verdict path argument asks for the root
// package alone.
func rootScopeArg(arg string) bool {
	return arg == "." || arg == "./"
}

// scopeVerdict narrows a verdict to the module's root package: the API and
// data model tables drop every symbol a subpackage declares, and the release
// is proposed again from what is left. A repository whose root package is its
// public API is judged on that package; a removal in a subpackage is a
// refactor there, not a breaking release.
//
// A release already made is a fact and keeps its version; only a proposal is
// made again.
func scopeVerdict(v verdict, dir string) (verdict, error) {
	v.Scope = scopeRoot
	v.API = rootOnlyDiff(v.Module, v.API)
	for hash, diff := range v.CommitAPI {
		v.CommitAPI[hash] = rootOnlyDiff(v.Module, diff)
	}
	v.Visibility = rootOnlyVisibility(v.Visibility)

	if v.Released {
		return v, nil
	}
	tags, _, err := moduleTags(dir)
	if err != nil {
		return verdict{}, err
	}
	return v.propose(tags)
}

// rootOnlyDiff drops every symbol that is not declared in the root package,
// and reads the breaking flag again from what is left: the one it arrived
// with was set over the whole module.
func rootOnlyDiff(module string, d apiDiff) apiDiff {
	if d.Skipped != "" {
		return d
	}

	d.Added = rootSymbols(module, d.Added)
	d.Removed = rootSymbols(module, d.Removed)

	changed := make([]apiChange, 0, len(d.Changed))
	for _, change := range d.Changed {
		if change.Package == module {
			changed = append(changed, change)
		}
	}
	d.Changed = changed

	types := make([]apiTypeChange, 0, len(d.Types))
	for _, change := range d.Types {
		if change.Package == module {
			types = append(types, change)
		}
	}
	d.Types = types

	d.Breaking = rootDiffBreaking(d)
	return d
}

// rootSymbols returns the symbols the root package declares.
func rootSymbols(module string, symbols []apiSymbol) []apiSymbol {
	kept := make([]apiSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		if symbol.Package == module {
			kept = append(kept, symbol)
		}
	}
	return kept
}

// rootDiffBreaking reports whether the narrowed diff still takes something
// from a consumer: an exported symbol removed, an exported signature changed,
// or a type whose exported fields moved against them.
func rootDiffBreaking(d apiDiff) bool {
	if len(d.ExportedRemoved()) > 0 {
		return true
	}
	for _, change := range d.Changed {
		if change.Exported {
			return true
		}
	}
	for _, change := range d.Types {
		if change.Breaking {
			return true
		}
	}
	return false
}

// rootOnlyVisibility keeps the one row the scope covers, which the report
// labels "./".
func rootOnlyVisibility(report visibilityReport) visibilityReport {
	if report.Skipped != "" {
		return report
	}
	kept := make([]visibilityPackage, 0, 1)
	for _, pkg := range report.Packages {
		if pkg.Package == "./" {
			kept = append(kept, pkg)
		}
	}
	report.Packages = kept
	return report
}
