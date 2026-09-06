// Package importfmt is the house rule for an import block, written as a
// function of the imports and nothing else.
//
// It reads no files and holds no state. The linter calls it to find out what a
// file's import declaration should look like, and the fixer calls it to write
// that. One function answering both is what keeps a rule that reports an error
// and a rewrite that clears it from disagreeing.
package importfmt

import (
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// The groups an import falls into, in the vocabulary goimports-reviser uses,
// so a project moving off it keeps the words it already writes in its
// pipeline.
const (
	// GroupStd is the standard library: a path whose first segment carries no
	// dot, which is the rule the toolchain itself applies.
	GroupStd = "std"

	// GroupBlanked is an import written for its side effect, and GroupDotted
	// one that puts the package's names in the file's scope. Neither is
	// reached by a name, so both are grouped by how they are written rather
	// than by where they come from.
	GroupBlanked = "blanked"
	GroupDotted  = "dotted"

	// GroupGeneral is a third party dependency, GroupCompany one under a
	// prefix the project calls its own, and GroupProject one of the module
	// being formatted.
	GroupGeneral = "general"
	GroupCompany = "company"
	GroupProject = "project"
)

// DefaultOrder is the order the groups are written in.
//
// It is the order six of the eight goimports-reviser call sites in this
// workspace pass, including the shared atkins go skill: blank imports sit
// second, directly under the standard library, where a reader looking for what
// a file registers finds them without reading the rest.
var DefaultOrder = []string{GroupStd, GroupBlanked, GroupGeneral, GroupCompany, GroupProject, GroupDotted}

// majorVersion matches the major version suffix of a module path, which is not
// part of the name the package is reached by.
var majorVersion = regexp.MustCompile(`/v[0-9]+$`)

// Options is the house rule as a value.
type Options struct {
	// Order is the groups, in the order they are written. A group left out is
	// written after the ones named, in DefaultOrder sequence, so an order that
	// forgets a group still writes every import.
	Order []string

	// Company are the path prefixes that form the company group. A trailing
	// slash is optional.
	Company []string

	// Project is the module path that forms the project group.
	Project string

	// SetAlias writes an alias for a path ending in a major version, so
	// "github.com/go-pg/pg/v9" is written as pg rather than read as v9.
	SetAlias bool
}

// NewOptions returns the defaults: the standard order, no company prefix, no
// project, and an alias on every versioned path.
func NewOptions() Options {
	return Options{Order: slices.Clone(DefaultOrder), SetAlias: true}
}

// order is the groups this run writes, every group named once.
func (o Options) order() []string {
	if len(o.Order) == 0 {
		return DefaultOrder
	}

	out := make([]string, 0, len(DefaultOrder))
	for _, group := range o.Order {
		if slices.Contains(DefaultOrder, group) && !slices.Contains(out, group) {
			out = append(out, group)
		}
	}
	for _, group := range DefaultOrder {
		if !slices.Contains(out, group) {
			out = append(out, group)
		}
	}

	return out
}

// Group is the group an import belongs to.
func Group(spec model.ImportSpec, opts Options) string {
	switch spec.Name {
	case ".":
		return GroupDotted
	case "_":
		return GroupBlanked
	}

	if standard(spec.Path) {
		return GroupStd
	}
	if under(spec.Path, opts.Project) {
		return GroupProject
	}
	for _, prefix := range opts.Company {
		if under(spec.Path, prefix) {
			return GroupCompany
		}
	}

	return GroupGeneral
}

// standard reports a path of the standard library, which is one whose first
// segment carries no dot. A domain has a dot in it and a standard library
// package does not, and that is the whole of the rule the toolchain applies.
func standard(importPath string) bool {
	first, _, _ := strings.Cut(importPath, "/")
	return !strings.Contains(first, ".")
}

// under reports a path equal to a prefix or below it. The prefix may be
// written with or without a trailing slash.
func under(importPath, prefix string) bool {
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		return false
	}
	return importPath == prefix || strings.HasPrefix(importPath, prefix+"/")
}

// Normalize is one import written the way the house writes it.
//
// An alias repeating the last segment of the path says nothing the path does
// not, and comes off. A path ending in a major version is reached by a name
// the path does not spell, so it gets one.
func Normalize(spec model.ImportSpec, opts Options) model.ImportSpec {
	switch spec.Name {
	case "_", ".":
		return spec
	case path.Base(spec.Path):
		spec.Name = ""
	}

	// A versioned path is reached by a name it does not spell, so it gets one,
	// but only where the path implies a name at all: what is left of
	// "example.com/v2" once the version comes off is "example.com", and
	// writing that as an alias produces a file that does not parse.
	if opts.SetAlias && spec.Name == "" && majorVersion.MatchString(spec.Path) {
		if name := model.BaseName(spec.Path); model.IsPackageName(name) {
			spec.Name = name
		}
	}

	return spec
}

// Canonical is the imports in the order and the form the house writes them:
// normalized, deduplicated, grouped and sorted within each group.
//
// Break is set on the first spec of every group after the first, so a caller
// rendering the result puts the blank lines where the groups end.
func Canonical(specs []model.ImportSpec, opts Options) []model.ImportSpec {
	order := opts.order()

	out := make([]model.ImportSpec, 0, len(specs))
	at := map[string]int{}

	for _, spec := range specs {
		spec = Normalize(spec, opts)
		spec.Break = false
		spec.Line = 0

		key := spec.Name + " " + spec.Path
		if index, seen := at[key]; seen {
			// The same import written twice keeps whatever either writing
			// said about it.
			if out[index].Doc == "" {
				out[index].Doc = spec.Doc
			}
			if out[index].Comment == "" {
				out[index].Comment = spec.Comment
			}
			continue
		}

		at[key] = len(out)
		out = append(out, spec)
	}

	sort.SliceStable(out, func(i, j int) bool {
		left, right := slices.Index(order, Group(out[i], opts)), slices.Index(order, Group(out[j], opts))
		if left != right {
			return left < right
		}
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Name < out[j].Name
	})

	previous := ""
	for i := range out {
		group := Group(out[i], opts)
		out[i].Break = i > 0 && group != previous
		previous = group
	}

	return out
}

// Format renders the imports as the declaration a file should hold.
//
// It is always the parenthesised form. A file writing one import on one line
// and a file writing six in a block then read the same, and two declarations
// merging into one has nowhere else to go.
func Format(specs []model.ImportSpec, opts Options) string {
	return Render(Canonical(specs, opts), "")
}

// Render writes the specs as a declaration, in the order they are given and
// with the aliases they carry, followed by the comment lines that belong to no
// import.
//
// It is the half of Format that does no deciding. A caller holding the imports
// a file actually writes renders them with this and compares the result
// against Format of the same imports: the two strings differ exactly when the
// file does not follow the rule.
func Render(specs []model.ImportSpec, tail string) string {
	if len(specs) == 0 {
		return ""
	}

	out := &strings.Builder{}
	out.WriteString("import (\n")

	for i, spec := range specs {
		if spec.Break && i > 0 {
			out.WriteString("\n")
		}
		for _, line := range docLines(spec.Doc) {
			out.WriteString(line)
			out.WriteString("\n")
		}
		out.WriteString("\t")
		out.WriteString(spec.Literal())
		if spec.Comment != "" {
			out.WriteString(" // ")
			out.WriteString(spec.Comment)
		}
		out.WriteString("\n")
	}

	for _, line := range docLines(tail) {
		out.WriteString(line)
		out.WriteString("\n")
	}

	out.WriteString(")")
	return out.String()
}

// docLines is a doc comment as the lines of an import block hold it: tab
// indented, one marker per line, and no trailing space on a line with nothing
// after the marker.
func docLines(doc string) []string {
	if doc == "" {
		return nil
	}

	lines := strings.Split(doc, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimRight(line, " \t"); line == "" {
			out = append(out, "\t//")
			continue
		}
		out = append(out, "\t// "+line)
	}

	return out
}

// Parse reads one import literal as the model records it: the quoted path,
// with the alias and a space in front of it when there is one.
func Parse(literal string) (model.ImportSpec, bool) {
	literal = strings.TrimSpace(literal)

	open := strings.IndexByte(literal, '"')
	if open < 0 {
		return model.ImportSpec{}, false
	}
	close := strings.LastIndexByte(literal, '"')
	if close <= open {
		return model.ImportSpec{}, false
	}

	importPath, err := strconv.Unquote(literal[open : close+1])
	if err != nil || importPath == "" {
		return model.ImportSpec{}, false
	}

	return model.ImportSpec{
		Name: strings.TrimSpace(literal[:open]),
		Path: importPath,
	}, true
}

// FormatLiterals renders a list of import literals as the declaration a file
// should hold. It is Format over the form Definition.Imports records, which is
// the form a caller holding a list of strings already has.
func FormatLiterals(literals []string, opts Options) string {
	specs := make([]model.ImportSpec, 0, len(literals))
	for _, literal := range literals {
		if spec, ok := Parse(literal); ok {
			specs = append(specs, spec)
		}
	}
	return Format(specs, opts)
}
