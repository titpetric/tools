package main

import (
	"sort"
	"strings"

	"github.com/titpetric/tools/worktree/components"
)

// symbolEntry is one symbol as a row of the API table.
type symbolEntry struct {
	category string
	pkg      string
	text     string

	// exported reports whether the symbol is API, which is the column the
	// text is written in: a reader looking for what a release costs reads
	// one column, and one reading a refactor reads the other.
	exported bool

	// commits are the short hashes of the commits that introduced or moved
	// the symbol, oldest first.
	commits []string
}

// symbolRows returns one row per symbol, in the order the report reads the
// categories: what the release adds first, what it takes away last.
//
// The category names only the first row of its group and the rows after it
// leave the column empty. The table draws no rule between rows, so a group
// reads as one block under its heading.
//
// The package a symbol belongs to has a column of its own, whatever the module
// holds: without it "const Name" says nothing about where it lives, and a
// module of one package still has to say which one that is. The symbols of a
// package are gathered together and only the first of them names it, the same
// way the data model table reads.
//
// A range read commit by commit gains a column naming the commits behind each
// symbol, which is what points a removal at the change behind it.
func symbolRows(v verdict, styled bool, wrap int) ([]string, [][]string) {
	touched := v.commitsBySymbol()

	var entries []symbolEntry
	for _, symbol := range v.API.Added {
		entries = append(entries, symbolEntry{"Added", symbol.Package, symbol.String(), symbol.Exported, touched[symbol.Key]})
		// An added type arrives with its exported fields and interface
		// methods; unlike a removal, what came along is worth reading.
		for _, field := range symbol.Fields {
			text := symbol.Name + "." + tidySignature(fieldReads(field))
			if field.Embedded {
				text = tidySignature(fieldReads(field))
			}
			entries = append(entries, symbolEntry{"Added", symbol.Package, text, symbol.Exported, touched[symbol.Key]})
		}
	}
	for _, change := range v.API.Changed {
		before := tidySignature(qualifySignature(change.Name, change.Old))
		after := tidySignature(qualifySignature(change.Name, change.New))
		entries = append(entries, symbolEntry{"Changed", change.Package, "Before: " + before + "\nAfter: " + after, change.Exported, touched[change.Key]})
	}
	for _, symbol := range collapseRemovedMethods(v.API.Removed) {
		entries = append(entries, symbolEntry{"Removed", symbol.Package, symbol.String(), symbol.Exported, touched[symbol.Key]})
	}
	if len(entries) == 0 {
		return nil, nil
	}

	headers := []string{"Change", "Package", "Exported", "Unexported"}
	widths := []int{len("Removed"), len("Package")}
	shortenPackages(entries, v)
	for _, entry := range entries {
		widths[1] = max(widths[1], len(entry.pkg))
	}
	groupByPackage(entries)

	hashes := make([][]string, 0, len(entries))
	for _, entry := range entries {
		hashes = append(hashes, entry.commits)
	}
	commits := commitCells(v, hashes, styled)
	if commits != nil {
		headers = append(headers, "Commits")
		widths = append(widths, columnWidth("Commits", commits))
	}
	// Two symbol columns share what one used to have.
	symbolWidth := cellWidth(wrap, append(widths, 0)) / 2

	var (
		rows                  [][]string
		lastCategory, lastPkg string
	)
	for i, entry := range entries {
		// A new category opens a group and names itself. The package is written
		// again under it, since a group opening on an empty cell says nothing.
		category := ""
		if entry.category != lastCategory {
			category = colorLines(entry.category, changeColor(entry.category), styled)
			lastCategory, lastPkg = entry.category, ""
		}

		pkg := entry.pkg
		if pkg == lastPkg {
			pkg = ""
		}
		lastPkg = entry.pkg

		row := []string{category, colorLines(pkg, components.ColorSeparator, styled)}
		text := fold(entry.text, symbolWidth)
		if entry.exported {
			row = append(row, text, "")
		} else {
			row = append(row, "", text)
		}
		if commits != nil {
			row = append(row, commits[i])
		}
		rows = append(rows, row)
	}
	return headers, rows
}

// groupByPackage gathers the symbols of a package together within their
// category, so a column naming the package can leave the repeats empty. The
// order within a package is the one the comparison reported.
func groupByPackage(entries []symbolEntry) {
	rank := map[string]int{"Added": 0, "Changed": 1, "Removed": 2}
	sort.SliceStable(entries, func(i, j int) bool {
		if a, b := rank[entries[i].category], rank[entries[j].category]; a != b {
			return a < b
		}
		return entries[i].pkg < entries[j].pkg
	})
}

// shortenPackages rewrites the package of every entry as its path below the
// module, which is how the table names it.
func shortenPackages(entries []symbolEntry, v verdict) {
	for i, entry := range entries {
		entries[i].pkg = shortPackage(v.Module, entry.pkg)
	}
}

// collapseRemovedMethods drops the methods and receiver-bound declarations of
// a type that is itself removed: the type row already says everything its
// methods would, since a type cannot go away and leave them behind.
func collapseRemovedMethods(symbols []apiSymbol) []apiSymbol {
	types := make(map[string]bool)
	for _, symbol := range symbols {
		if symbol.Kind == "type" {
			types[symbol.Package+"\x00"+symbol.Name] = true
		}
	}
	kept := make([]apiSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		if receiver := receiverType(symbol.Name); receiver != "" && types[symbol.Package+"\x00"+receiver] {
			continue
		}
		kept = append(kept, symbol)
	}
	return kept
}

// receiverType returns the type a qualified name hangs off ("Disk.Save" is
// hung off "Disk"), or an empty string for a package level name.
func receiverType(name string) string {
	receiver, _, found := strings.Cut(name, ".")
	if !found {
		return ""
	}
	return receiver
}
