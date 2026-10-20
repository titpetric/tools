package config

import (
	"slices"
	"strings"
)

// Field is one editable setting of a configuration document. The field holds
// its value as a pointer into the Config it came from, so a saved form writes
// straight through into the document.
//
// Exactly one of Bool and List holds a value; IsList reports which.
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

// IsList reports whether the field holds a string list. A false result means
// the field holds a boolean.
func (f Field) IsList() bool {
	return f.List != nil
}

// value is the edited state of one setting. The form keeps one per field and
// writes them into the document only when it saves, so leaving the form
// discards the edits and the document needs no reload.
type value struct {
	// flag is the state of a boolean setting.
	flag bool

	// text is a string list as the user types it, entries separated by commas.
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

// Fields returns every editable setting, flattened out of its section.
func (c *Config) Fields() []Field {
	var fields []Field
	for _, section := range c.Sections() {
		fields = append(fields, section.Fields...)
	}
	return fields
}
