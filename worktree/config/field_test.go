package config

import (
	"reflect"
	"strings"
	"testing"
)

// TestFieldsCoverEverySetting checks the form reaches every setting the
// document holds, so nothing can only be changed by editing the file.
func TestFieldsCoverEverySetting(t *testing.T) {
	cfg := Default()
	data, err := Encode(cfg)
	if err != nil {
		t.Fatal(err)
	}
	document := string(data)

	seen := make(map[string]bool)
	for _, field := range cfg.Fields() {
		if field.Title == "" || field.Help == "" {
			t.Fatalf("field %q is missing a title or a description", field.Key)
		}
		// The description sits on the row of the setting it describes, so it
		// has to be one short line.
		if len(field.Help) > 40 || strings.Contains(field.Help, "\n") {
			t.Fatalf("field %q describes itself in %q, want one short line", field.Key, field.Help)
		}
		if (field.Bool == nil) == (field.List == nil) {
			t.Fatalf("field %q must hold exactly one of a boolean and a list", field.Key)
		}
		if seen[field.Key] {
			t.Fatalf("field %q appears twice", field.Key)
		}
		seen[field.Key] = true

		// The key is dotted for display; the document holds its last segment.
		name := field.Key[strings.LastIndex(field.Key, ".")+1:]
		if !strings.Contains(document, name+":") {
			t.Fatalf("field %q names %q, which the document does not hold:\n%s", field.Key, name, document)
		}
	}

	// version is written but not editable, so every other key needs a field.
	for _, key := range []string{"enable_gitignore", "enable_git_repos", "ignore_paths", "root_markers"} {
		if !seen["scan."+key] {
			t.Fatalf("no field edits scan.%s", key)
		}
	}
}

// TestFieldStateReadsTheDocument checks a field starts on the value the
// document holds, which is what the form opens on.
func TestFieldStateReadsTheDocument(t *testing.T) {
	cfg := Default()
	fields := cfg.Fields()

	for _, field := range fields {
		got := field.state()
		switch field.Key {
		case "scan.enable_gitignore", "scan.enable_git_repos":
			if !got.flag {
				t.Fatalf("state() of %q is off, want the document value", field.Key)
			}
		case "scan.ignore_paths":
			if got.text != "" {
				t.Fatalf("state() of %q = %q, want an empty list to read as empty text", field.Key, got.text)
			}
		case "scan.root_markers":
			if want := "go.work, go.mod, .git"; got.text != want {
				t.Fatalf("state() of %q = %q, want %q", field.Key, got.text, want)
			}
		}
	}
}

// TestFieldApplyWritesTheDocument checks a saved form reaches the document.
func TestFieldApplyWritesTheDocument(t *testing.T) {
	cfg := &Config{}

	for _, field := range cfg.Fields() {
		if field.IsList() {
			field.apply(value{text: " added , second ,,"})
			continue
		}
		field.apply(value{flag: true})
	}

	if !cfg.Scan.EnableGitignore || !cfg.Scan.EnableGitRepos {
		t.Fatalf("apply() did not reach the document: %#v", cfg.Scan)
	}
	want := []string{"added", "second"}
	if !reflect.DeepEqual(cfg.Scan.IgnorePaths, want) || !reflect.DeepEqual(cfg.Scan.RootMarkers, want) {
		t.Fatalf("apply() did not reach the list settings: %#v", cfg.Scan)
	}
}

func TestValueEntries(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{"empty", "", nil},
		{"one entry", "go.mod", []string{"go.mod"}},
		{"two entries", "go.work, go.mod", []string{"go.work", "go.mod"}},
		{"no spacing", "go.work,go.mod", []string{"go.work", "go.mod"}},
		{"separator half typed", "go.work, ", []string{"go.work"}},
		{"blanks dropped", " , go.mod ,, .git ", []string{"go.mod", ".git"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := (value{text: test.text}).entries(); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("entries() = %v, want %v", got, test.want)
			}
		})
	}
}

// TestValueEqual checks an edit is measured by what the document would hold,
// so retyping a list the same way is not a change to save.
func TestValueEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b value
		list bool
		want bool
	}{
		{"same flag", value{flag: true}, value{flag: true}, false, true},
		{"flag toggled", value{flag: true}, value{}, false, false},
		{"same list", value{text: "a, b"}, value{text: "a, b"}, true, true},
		{"respaced list", value{text: "a, b"}, value{text: "a,b "}, true, true},
		{"separator half typed", value{text: "a, b"}, value{text: "a, b,"}, true, true},
		{"entry added", value{text: "a, b"}, value{text: "a, b, c"}, true, false},
		{"entry reordered", value{text: "a, b"}, value{text: "b, a"}, true, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.a.equal(test.b, test.list); got != test.want {
				t.Fatalf("equal() = %v, want %v", got, test.want)
			}
		})
	}
}
