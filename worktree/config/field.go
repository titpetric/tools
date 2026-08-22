package config

import (
	"slices"
	"strings"
)

// Field is one editable setting of a configuration document. The value is
// held as a pointer into the Config the field was built from, so a saved form
// writes straight through into the document.
//
// Exactly one of Bool and List is set, which is what IsList reports.
type Field struct {
	// Title is the label the form shows.
	Title string

	// Key is the name the setting has in the document.
	Key string

	// Help is the one line description shown beside the value.
	Help string

	// Bool points at a boolean setting, or is nil.
	Bool *bool

	// List points at a string list setting, or is nil.
	List *[]string
}

// IsList reports whether the field holds a string list rather than a boolean.
func (f Field) IsList() bool {
	return f.List != nil
}

// value is the edited state of one setting. The form keeps one per field and
// writes them into the document only when it saves, so leaving the form
// discards the edits rather than the document having to be reloaded.
type value struct {
	// flag is the state of a boolean setting.
	flag bool

	// text is a string list as it is typed, entries separated by commas.
	text string
}

// entries splits list text into the entries the document holds, dropping the
// blanks that typing a separator leaves behind.
func (v value) entries() []string {
	var entries []string
	for _, entry := range strings.Split(v.text, ",") {
		if entry = strings.TrimSpace(entry); entry != "" {
			entries = append(entries, entry)
		}
	}
	return entries
}

// equal compares two values as the document would hold them, so respacing a
// list, or typing a separator that adds no entry, is not an edit.
func (v value) equal(other value, list bool) bool {
	if !list {
		return v.flag == other.flag
	}
	return slices.Equal(v.entries(), other.entries())
}

// state reads the setting out of the document, the value the form starts on.
func (f Field) state() value {
	if f.IsList() {
		return value{text: strings.Join(*f.List, ", ")}
	}
	return value{flag: *f.Bool}
}

// apply writes an edited value back into the document.
func (f Field) apply(v value) {
	if f.IsList() {
		*f.List = v.entries()
		return
	}
	*f.Bool = v.flag
}

// Section is a named group of settings, one form heading.
type Section struct {
	Title  string
	Fields []Field
}

// Sections returns the editable settings of the document, in the order the
// form shows them. Every setting the document holds appears exactly once, so
// the form covers the whole file.
func (c *Config) Sections() []Section {
	return []Section{
		{
			Title: "Scan",
			Fields: []Field{
				{
					Title: "Enable Gitignore",
					Key:   "scan.enable_gitignore",
					Bool:  &c.Scan.EnableGitignore,
					Help:  "Skip what a .gitignore excludes",
				},
				{
					Title: "Enable Git Repos",
					Key:   "scan.enable_git_repos",
					Bool:  &c.Scan.EnableGitRepos,
					Help:  "List git repos without a go.mod",
				},
				{
					Title: "Ignore Paths",
					Key:   "scan.ignore_paths",
					List:  &c.Scan.IgnorePaths,
					Help:  "Folder names never descended into",
				},
				{
					Title: "Root Markers",
					Key:   "scan.root_markers",
					List:  &c.Scan.RootMarkers,
					Help:  "Files marking the workspace root",
				},
			},
		},
	}
}

// Fields returns every editable setting, flattened out of its section.
func (c *Config) Fields() []Field {
	var fields []Field
	for _, section := range c.Sections() {
		fields = append(fields, section.Fields...)
	}
	return fields
}
