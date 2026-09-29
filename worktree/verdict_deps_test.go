package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// sampleDeps is the dependency movement of a release that did all three things
// at once, in the order the report reads them.
func sampleDeps() []DiffResult {
	return []DiffResult{
		{Path: "github.com/google/go-cmp", Change: fieldAdded, New: "v0.7.0"},
		{Path: "golang.org/x/sync", Change: fieldAdded, New: "v0.23.0"},
		{Path: "golang.org/x/mod", Change: fieldChanged, Old: "v0.40.0", New: "v0.41.0"},
		{Path: "gopkg.in/yaml.v3", Change: fieldRemoved, Old: "v3.0.1"},
	}
}

func TestRenderVerdictDependencies(t *testing.T) {
	v := sampleVerdict()
	v.API.Deps = sampleDeps()

	var out bytes.Buffer
	renderVerdict(&out, v, false)

	got := out.String()
	for _, want := range []string{
		"## Dependencies since v1.0.0",
		"| Change | Module | Version |",
		"| Added | github.com/google/go-cmp | v0.7.0 |",
		// The category opens its group and the row under it leaves the cell
		// empty, the way the API and data model tables read.
		"|  | golang.org/x/sync | v0.23.0 |",
		// A move is written as the two versions either side of it, the same
		// way the data model writes a field that changed shape.
		"| Changed | golang.org/x/mod | v0.40.0 -> v0.41.0 |",
		"| Removed | gopkg.in/yaml.v3 | v3.0.1 |",
		// The release is still a minor for what it took out of the API, and
		// the dependencies are named after the reason rather than as one.
		"Minor release: v1.1.0, because 1 exported symbol was removed",
		"2 dependencies were added and 1 dependency moved to another version and 1 dependency was dropped.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
		}
	}

	// The section sits between the data model and the working tree, so the
	// report reads the range through and then describes the tree as it stands.
	deps, visibility := strings.Index(got, "## Dependencies"), strings.Index(got, "## Visibility")
	if model := strings.Index(got, "## Data model"); !(model < deps && deps < visibility) {
		t.Errorf("renderVerdict() ordered the sections [data model %d, dependencies %d, visibility %d]", model, deps, visibility)
	}
}

// TestRenderVerdictOmitsDependenciesWhenNoneMoved pins that a release touching
// no go.mod writes no section, the way one touching no type writes no data
// model table.
func TestRenderVerdictOmitsDependenciesWhenNoneMoved(t *testing.T) {
	var out bytes.Buffer
	renderVerdict(&out, sampleVerdict(), false)

	got := out.String()
	if strings.Contains(got, "Dependencies") {
		t.Errorf("renderVerdict() wrote a dependency section for a release that moved none:\n%s", got)
	}
	if strings.Contains(got, "dependency") || strings.Contains(got, "dependencies") {
		t.Errorf("renderVerdict() named dependencies in the summary of a release that moved none:\n%s", got)
	}
}

func TestRenderVerdictDependenciesANSI(t *testing.T) {
	v := sampleVerdict()
	v.API.Deps = sampleDeps()

	var out bytes.Buffer
	renderVerdict(&out, v, true)

	got := out.String()
	for _, unwanted := range []string{"| Change | Module | Version |", "## Dependencies"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("renderVerdict() wrote markup %q to a terminal:\n%s", unwanted, got)
		}
	}
	if !strings.Contains(got, "github.com/google/go-cmp") {
		t.Errorf("renderVerdict() left the dependencies out of a terminal report:\n%s", got)
	}
}

func TestVerdictDepNote(t *testing.T) {
	tests := []struct {
		name string
		in   []DiffResult
		want string
	}{
		{name: "nothing moved"},
		{
			name: "one added",
			in:   []DiffResult{{Path: "a", Change: fieldAdded, New: "v1.0.0"}},
			want: " 1 dependency was added.",
		},
		{
			name: "one of each",
			in: []DiffResult{
				{Path: "a", Change: fieldAdded, New: "v1.0.0"},
				{Path: "b", Change: fieldChanged, Old: "v1.0.0", New: "v1.1.0"},
				{Path: "c", Change: fieldRemoved, Old: "v1.0.0"},
			},
			want: " 1 dependency was added and 1 dependency moved to another version and 1 dependency was dropped.",
		},
		{
			name: "several dropped",
			in: []DiffResult{
				{Path: "a", Change: fieldRemoved, Old: "v1.0.0"},
				{Path: "b", Change: fieldRemoved, Old: "v1.0.0"},
			},
			want: " 2 dependencies were dropped.",
		},
	}

	for _, test := range tests {
		v := verdict{API: apiDiff{Deps: test.in}}
		if got := v.depNote(); got != test.want {
			t.Errorf("%s: depNote() = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestModelRequires(t *testing.T) {
	root := t.TempDir()

	document := `{"SchemaVersion":1,"Modules":[{"Path":"example.com/app","Requires":[
		{"Path":"golang.org/x/mod","Version":"v0.41.0"},
		{"Path":"golang.org/x/sync","Version":"v0.23.0","Indirect":true}]}],"Packages":[]}`
	// A document written before there was a root is a bare list of packages,
	// each carrying the module it came from.
	bare := `[{"ImportPath":"example.com/app","Module":{"Path":"example.com/app","Requires":[
		{"Path":"golang.org/x/mod","Version":"v0.41.0"},
		{"Path":"golang.org/x/sync","Version":"v0.23.0","Indirect":true}]}}]`

	want := []requireInfo{
		{path: "golang.org/x/mod", version: "v0.41.0"},
		{path: "golang.org/x/sync", version: "v0.23.0", indirect: true},
	}
	for name, data := range map[string]string{"document.json": document, "bare.json": bare} {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if got := modelRequires(path); !reflect.DeepEqual(got, want) {
			t.Errorf("modelRequires(%s) = %+v, want %+v", name, got, want)
		}
	}

	// The empty document a first release is measured against, a model written
	// without a go.mod, an unreadable one, and a file that is not there: every
	// one of them reports no requirement rather than failing the run.
	for name, data := range map[string]string{
		"empty.json":    "[]\n",
		"nomodule.json": `{"SchemaVersion":1,"Packages":[{"ImportPath":"example.com/app"}]}`,
		"notjson.json":  "this is not a document",
		"absent-marker": "",
	} {
		path := filepath.Join(root, name)
		if data != "" {
			if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
		}
		if got := modelRequires(path); len(got) != 0 {
			t.Errorf("modelRequires(%s) = %+v, want nothing", name, got)
		}
	}
}
