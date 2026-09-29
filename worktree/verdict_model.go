package main

import (
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/titpetric/tools/worktree/components"
)

// newTypeMark flags a type the release introduces, so a field of a new type
// reads apart from one added to a type that was already there. It is the mark
// the dependency matrix uses, and for the same reason: something that is there
// now and was not before.
const newTypeMark = dependencyMark

// fieldEntry is one exported field as a row of the data model table.
type fieldEntry struct {
	category string
	pkg      string
	typeName string

	// newType reports that the type itself is new, rather than one that was
	// already there gaining a field.
	newType bool

	text string

	// commits are the short hashes of the commits that introduced or moved
	// the field, oldest first.
	commits []string
}

// writeDataModel writes what became of the exported fields of every type the
// release touches, which is the shape of the data a consumer reads and writes.
//
// It is one table rather than one per type: a release touching a dozen types
// otherwise writes a dozen tables that line up with none of the others.
func writeDataModel(w io.Writer, v verdict, styled bool, wrap int) {
	headers, rows := dataModelRows(v, styled, wrap)
	if len(rows) == 0 {
		return
	}
	writeHeading(w, "Data model "+v.Range(), styled)
	writeSimpleTable(w, headers, rows, styled)
	writeGap(w, styled)
}

// dataModelRows returns one row per field, in the order the report reads the
// categories: what the release adds first, what it takes away last.
//
// A cell repeating the one above it is left empty, so a type reads as one block
// of its fields. The category starts a group and the package and type are
// restated under it, since a group opening on three empty cells says nothing.
func dataModelRows(v verdict, styled bool, wrap int) ([]string, [][]string) {
	entries := dataModelEntries(v)
	if len(entries) == 0 {
		return nil, nil
	}

	headers := []string{"Change", "Package", "Type", "Field"}
	widths := []int{len("Removed"), len("Package"), len("Type")}
	for _, entry := range entries {
		widths[1] = max(widths[1], len(entry.pkg))
		widths[2] = max(widths[2], len(entry.typeName)+len(" "+newTypeMark))
	}

	hashes := make([][]string, 0, len(entries))
	for _, entry := range entries {
		hashes = append(hashes, entry.commits)
	}
	commits := commitCells(v, hashes, styled)
	if commits != nil {
		headers = append(headers, "Commits")
		widths = append(widths, columnWidth("Commits", commits))
	}
	fieldWidth := cellWidth(wrap, append(widths, 0))

	var (
		rows                            [][]string
		lastCategory, lastPkg, lastType string
	)
	for i, entry := range entries {
		// A new category opens a group, and names itself. The package and type
		// are written again under it, however far the group above them reached,
		// since a group opening on empty cells says nothing.
		category := ""
		if entry.category != lastCategory {
			category = entry.category
			lastCategory, lastPkg, lastType = entry.category, "", ""
		}

		row := dataModelRow(entry, category, styled, fieldWidth)
		if entry.pkg == lastPkg {
			row[1] = ""
			if entry.typeName == lastType {
				row[2] = ""
			}
		}
		if commits != nil {
			row = append(row, commits[i])
		}

		lastPkg, lastType = entry.pkg, entry.typeName
		rows = append(rows, row)
	}
	return headers, rows
}

// dataModelRow renders one field as the cells of a row, with the category named
// only when it opens a group.
func dataModelRow(entry fieldEntry, category string, styled bool, width int) []string {
	if category != "" {
		category = colorLines(categoryName(category), changeColor(category), styled)
	}

	name := entry.typeName
	if entry.newType {
		name += " " + colorLines(newTypeMark, components.ColorGreen, styled)
	}

	// A field added to a type that already existed carries the mark itself;
	// on a new type the mark on the type name already covers every field.
	text := entry.text
	if entry.category == fieldAdded && !entry.newType {
		text += " " + colorLines(newTypeMark, components.ColorGreen, styled)
	}

	return []string{
		category,
		colorLines(entry.pkg, components.ColorSeparator, styled),
		name,
		fold(text, width),
	}
}

// dataModelEntries returns every field the release moved, the added ones first,
// and within a category ordered by package, type and name.
//
// The fields of a type the release adds are read as additions of their own, so
// the report carries the shape it declares rather than only that it exists.
// Unexported members are left out: they are not part of the API.
func dataModelEntries(v verdict) []fieldEntry {
	touched := v.commitsByField()

	var entries []fieldEntry
	add := func(key, pkg, typeName string, newType bool, change apiFieldChange) {
		entries = append(entries, fieldEntry{
			category: change.Change,
			pkg:      shortPackage(v.Module, pkg),
			typeName: typeName,
			newType:  newType,
			text:     fieldText(change),
			commits:  touched[fieldKey(key, change.Name)],
		})
	}

	for _, symbol := range addedTypes(v.API) {
		for _, field := range symbol.Fields {
			add(symbol.Key, symbol.Package, typeReads(symbol.Name, symbol.Underlying), true, apiFieldChange{
				Name: field.Name, Change: fieldAdded, New: &field,
			})
		}
	}
	for _, want := range []string{fieldAdded, fieldChanged, fieldRemoved} {
		for _, change := range v.API.Types {
			for _, field := range change.Fields {
				if field.Change == want {
					add(change.Key, change.Package, typeReads(change.Name, change.Underlying), false, field)
				}
			}
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.category != b.category {
			return categoryOrder(a.category) < categoryOrder(b.category)
		}
		if a.pkg != b.pkg {
			return a.pkg < b.pkg
		}
		if a.typeName != b.typeName {
			return a.typeName < b.typeName
		}
		return a.text < b.text
	})
	return entries
}

// typeReads names a type with the shape it is declared with.
func typeReads(name, underlying string) string {
	if underlying == "" {
		return "type " + name
	}
	return "type " + name + " " + underlying
}

// addedTypes returns the types the release adds that declare an exported field,
// which is what there is to write a shape for. A type with none, such as a func
// type or a named string, is already said by its row in the API table.
func addedTypes(diff apiDiff) []apiSymbol {
	var types []apiSymbol
	for _, symbol := range diff.Added {
		if symbol.Kind == "type" && len(symbol.Fields) > 0 {
			types = append(types, symbol)
		}
	}
	return types
}

// fieldText renders a field as the table lists it: its name, and the shape it
// has or the two shapes it moved between.
//
// The name leads, since it has no column of its own, and is written once for a
// field that moved: a field is matched to the one it was by name, so the name
// is the one thing that cannot have changed.
//
// A tag is part of the shape, since it is what a stored document decodes
// through, and is written alongside the type whenever either side carries one.
//
// A change to the tag alone is named as one, "Children: added json tag":
// writing the unchanged type on both sides of an arrow buries what moved, and
// reads as a field losing its name.
func fieldText(change apiFieldChange) string {
	switch {
	case change.Old != nil && change.New != nil:
		if change.Old.Embedded {
			return "embeds " + change.Old.Type + " -> " + change.New.Type
		}
		if change.Old.Type == change.New.Type && change.Old.Tag != change.New.Tag {
			if moved := tagChange(change.Old.Tag, change.New.Tag); moved != "" {
				return change.Name + ": " + moved
			}
		}
		return tidySignature(fieldLabel(change.Name, fieldShape(*change.Old)) + " -> " + fieldShape(*change.New))
	case change.New != nil:
		return tidySignature(fieldReads(*change.New))
	case change.Old != nil:
		return tidySignature(fieldReads(*change.Old))
	}
	return change.Name
}

// tagChange names what moved between two struct tags, key by key: a key
// added, a key removed, or the value under one rewritten. It returns nothing
// when either tag does not parse, and the caller falls back to writing the
// two shapes whole.
func tagChange(oldTag, newTag string) string {
	oldKeys, oldValues, ok := tagEntries(oldTag)
	if !ok {
		return ""
	}
	newKeys, newValues, ok := tagEntries(newTag)
	if !ok {
		return ""
	}

	var parts []string
	for _, key := range newKeys {
		before, had := oldValues[key]
		switch {
		case !had:
			parts = append(parts, "added "+key+" tag")
		case before != newValues[key]:
			parts = append(parts, key+" tag "+strconv.Quote(before)+" -> "+strconv.Quote(newValues[key]))
		}
	}
	for _, key := range oldKeys {
		if _, has := newValues[key]; !has {
			parts = append(parts, "removed "+key+" tag")
		}
	}
	return strings.Join(parts, ", ")
}

// tagEntries reads a struct tag into its keys, written in order, and the
// value each maps to. The format is the one reflect.StructTag reads: a name,
// a colon, a quoted value, over and over. A tag written any other way reports
// false, and an empty tag is a tag with no keys.
func tagEntries(tag string) (keys []string, values map[string]string, ok bool) {
	values = map[string]string{}

	for tag != "" {
		tag = strings.TrimLeft(tag, " ")
		if tag == "" {
			break
		}

		colon := strings.IndexByte(tag, ':')
		if colon <= 0 || strings.ContainsAny(tag[:colon], " \"") {
			return nil, nil, false
		}
		name := tag[:colon]
		rest := tag[colon+1:]
		if !strings.HasPrefix(rest, `"`) {
			return nil, nil, false
		}

		value, err := strconv.QuotedPrefix(rest)
		if err != nil {
			return nil, nil, false
		}
		tag = rest[len(value):]

		unquoted, err := strconv.Unquote(value)
		if err != nil {
			return nil, nil, false
		}
		keys = append(keys, name)
		values[name] = unquoted
	}

	return keys, values, true
}

// fieldReads renders one field, an embed as the type it embeds.
func fieldReads(field apiField) string {
	if field.Embedded {
		return "embeds " + field.Type
	}
	return fieldLabel(field.Name, fieldShape(field))
}

// fieldLabel writes the name in front of a shape, unless the shape opens on it
// already. An interface method carries its name in the signature it is recorded
// under, as "Put (key string) error", where the type of a struct field does
// not.
//
// The parameter list is what the name is recognised by, not the name alone: a
// field is often typed after itself, and "Mode Mode" is a field named Mode of
// type Mode rather than a name written twice.
func fieldLabel(name, shape string) string {
	if strings.HasPrefix(shape, name+" (") {
		return shape
	}
	return name + " " + shape
}

// fieldShape renders one side of a field, which is its type and the tag it
// carries.
func fieldShape(field apiField) string {
	if field.Tag == "" {
		return field.Type
	}
	return field.Type + " `" + field.Tag + "`"
}
