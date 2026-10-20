package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
)

// modelModule is the go.mod a splint model records for its source module.
// This file reads only the requirements: the verdict compares the go directive
// on its own, and the go directive decides the release where a require line
// does not.
type modelModule struct {
	Requires []struct {
		Path     string
		Version  string
		Indirect bool
	}
}

// modelDocument is the shape of a splint model on disk; this file reads only
// its modules.
type modelDocument struct {
	Modules  []modelModule
	Packages []struct {
		Module *modelModule
	}
}

// modelRequires returns the requirements the go.mod of a splint model records.
//
// A model carrying none reports none, which covers the two cases a comparison
// meets: the empty document a first release compares against, and a model
// from before the go.mod joined it. Either way the report covers what the
// other side holds, the same way a first release reports every symbol it
// exports as added.
//
// A document from before the root existed is a bare list of packages, each
// carrying its source module, and modelRequires reads it the way splint's own
// loader reads it.
func modelRequires(model string) []requireInfo {
	data, err := os.ReadFile(model)
	if err != nil {
		return nil
	}

	var doc modelDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		var packages []struct {
			Module *modelModule
		}
		if err := json.Unmarshal(data, &packages); err != nil {
			return nil
		}
		doc.Packages = packages
	}

	module, ok := firstModule(doc)
	if !ok {
		return nil
	}

	reqs := make([]requireInfo, 0, len(module.Requires))
	for _, r := range module.Requires {
		reqs = append(reqs, requireInfo{path: r.Path, version: r.Version, indirect: r.Indirect})
	}
	return reqs
}

// firstModule returns the go.mod the document describes. Extraction records the
// module on every package it produced and again at the root, so the rest
// repeat the first one firstModule finds.
func firstModule(doc modelDocument) (modelModule, bool) {
	if len(doc.Modules) > 0 {
		return doc.Modules[0], true
	}
	for _, pkg := range doc.Packages {
		if pkg.Module != nil {
			return *pkg.Module, true
		}
	}
	return modelModule{}, false
}

// writeDependencies writes what the release changes about depending on the
// module, apart from calling it.
//
// Nothing here earns a release a minor: a moved require line takes nothing away
// that a compiler will complain about. It still belongs in the note, since it
// decides what a consumer downloads, which lowest versions its own build has
// to meet, and whose code lands in the binary.
func writeDependencies(w io.Writer, v verdict, styled bool) {
	headers, rows := dependencyRows(v, styled)
	if len(rows) == 0 {
		return
	}
	writeHeading(w, "Dependencies "+v.Range(), styled)
	writeSimpleTable(w, headers, rows, styled)
	writeGap(w, styled)
}

// dependencyRows returns one row per required module, in the order the report reads
// the categories: what the release adds first, what it drops last.
//
// The category names the first row of its group and the rows under it leave the
// cell empty, the way the API and data model tables read. The module path
// appears in full: it names the dependency itself, and says nothing of where
// it lives.
func dependencyRows(v verdict, styled bool) ([]string, [][]string) {
	if len(v.API.Deps) == 0 {
		return nil, nil
	}

	var (
		rows         [][]string
		lastCategory string
	)
	for _, change := range v.API.Deps {
		category := ""
		if change.Change != lastCategory {
			category = colorLines(categoryName(change.Change), changeColor(change.Change), styled)
			lastCategory = change.Change
		}
		rows = append(rows, []string{
			category,
			change.Path,
			colorLines(change.Version(), changeColor(change.Change), styled),
		})
	}
	return []string{"Change", "Module", "Version"}, rows
}

// depNote names the dependency movement, as the sentence that follows the
// verdict. A dependency is not API, so it explains the release and does not
// decide it; a release that moved none reads exactly as it did before.
func (v verdict) depNote() string {
	var added, changed, removed int
	for _, change := range v.API.Deps {
		switch change.Change {
		case fieldAdded:
			added++
		case fieldChanged:
			changed++
		case fieldRemoved:
			removed++
		}
	}
	if added+changed+removed == 0 {
		return ""
	}

	var parts []string
	if added > 0 {
		parts = append(parts, plural(added, "dependency was added", "dependencies were added"))
	}
	if changed > 0 {
		parts = append(parts, plural(changed, "dependency moved to another version", "dependencies moved to another version"))
	}
	if removed > 0 {
		parts = append(parts, plural(removed, "dependency was dropped", "dependencies were dropped"))
	}
	return " " + strings.Join(parts, " and ") + "."
}
