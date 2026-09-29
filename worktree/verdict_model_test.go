package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderVerdictNamesInterfaceMethods(t *testing.T) {
	var out bytes.Buffer
	renderVerdict(&out, sampleInterfaceChange(), false)

	got := out.String()
	// A method needs no treatment of its own: the type column names the
	// interface and the field column carries the signature, so the row reads as
	// store.Store.Put.
	if want := "| Added | /store | type Store interface | Put (key string) error ▲ |"; !strings.Contains(got, want) {
		t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
	}
}

func TestRenderVerdictOmitsTheDataModelWhenNothingMoved(t *testing.T) {
	v := sampleVerdict()
	v.API.Added, v.API.Types = nil, nil

	var out bytes.Buffer
	renderVerdict(&out, v, false)

	if got := out.String(); strings.Contains(got, "Data model") {
		t.Errorf("renderVerdict() wrote a data model section with nothing in it:\n%s", got)
	}
}

func TestRenderVerdictWritesNoShapeForATypeWithoutFields(t *testing.T) {
	v := sampleVerdict()
	// A func type declares no field, so its row in the API table says all
	// there is to say about it.
	v.API.Added = []apiSymbol{{
		Key: "example.com/x.Option", Package: "example.com/x",
		Name: "Option", Kind: "type", Exported: true, Underlying: "func(*Client)",
	}}
	v.API.Types = nil

	var out bytes.Buffer
	renderVerdict(&out, v, false)

	// The API table says the type is there; there is no shape to write for it,
	// so the section is left out entirely.
	got := out.String()
	if !strings.Contains(got, "| Added | / | type Option func(*Client) |") {
		t.Errorf("renderVerdict() lost the type from the API table:\n%s", got)
	}
	if strings.Contains(got, "Data model") {
		t.Errorf("renderVerdict() wrote a data model section for a type declaring no field:\n%s", got)
	}
}

func TestRenderVerdictDataModelIsOneTable(t *testing.T) {
	v := sampleVerdict()
	// A second package, so the table has something to keep apart.
	v.API.Types = append(v.API.Types, apiTypeChange{
		Key: "example.com/x/inner.Store", Package: "example.com/x/inner",
		Name: "Store", Underlying: "struct", Breaking: true,
		Fields: []apiFieldChange{
			{Name: "Bucket", Change: fieldAdded, New: &apiField{Name: "Bucket", Type: "string"}},
			{Name: "Region", Change: fieldRemoved, Old: &apiField{Name: "Region", Type: "string"}},
		},
	})

	var out bytes.Buffer
	renderVerdict(&out, v, false)
	got := out.String()

	// One table for every type the release touched, and no heading per type.
	if n := strings.Count(got, "| Change | Package | Type | Field |"); n != 1 {
		t.Errorf("renderVerdict() wrote %d data model tables, want 1:\n%s", n, got)
	}
	if strings.Contains(got, "###") {
		t.Errorf("renderVerdict() wrote a heading per type:\n%s", got)
	}

	// The rows the table holds, in order: added first, taken away last, and
	// within a category by package, then type, then field.
	// The category names only the first row of its group, as it does in the API
	// table above.
	want := []string{
		"| Added | / | type Client struct ▲ | Name string `json:\"name\"` |",
		"|  |  | type Config struct | Timeout int ▲ |",
		"|  | /inner | type Store struct | Bucket string ▲ |",
		"| Changed | / | type Config struct | Addr string `yaml:\"addr\"` -> []string `yaml:\"addr\"` |",
		"| Removed | / | type Config struct | Retries int |",
		"|  | /inner | type Store struct | Region string |",
	}
	at := -1
	for _, row := range want {
		next := strings.Index(got, row)
		if next < 0 {
			t.Fatalf("renderVerdict() output missing %q:\n%s", row, got)
		}
		if next < at {
			t.Errorf("renderVerdict() wrote %q out of order:\n%s", row, got)
		}
		at = next
	}

	// A category opens a group, and restates the package and type under it
	// however far the group above them reached.
	if strings.Contains(got, "| Removed |  |") {
		t.Errorf("renderVerdict() opened a category on an empty package cell:\n%s", got)
	}
	// The added type carries the mark once, and each field added to a type
	// that already existed carries it on the field; fields of the new type
	// do not repeat it.
	if n := strings.Count(got, newTypeMark); n != 3 {
		t.Errorf("renderVerdict() wrote the mark %d times, want 3:\n%s", n, got)
	}
}

func TestFieldText(t *testing.T) {
	tests := []struct {
		title  string
		change apiFieldChange
		want   string
	}{{
		title:  "a field that was added has the shape it arrived with",
		change: apiFieldChange{Name: "Retries", Change: fieldAdded, New: &apiField{Name: "Retries", Type: "int"}},
		want:   "Retries int",
	}, {
		title:  "a field that was removed has the shape it left with",
		change: apiFieldChange{Name: "Retries", Change: fieldRemoved, Old: &apiField{Name: "Retries", Type: "int"}},
		want:   "Retries int",
	}, {
		// The name is written once: a field is matched to the one it was by
		// name, so the name is the one thing that cannot have changed.
		title: "a field that moved has both shapes under one name",
		change: apiFieldChange{
			Name: "Addr", Change: fieldChanged,
			Old: &apiField{Name: "Addr", Type: "string"},
			New: &apiField{Name: "Addr", Type: "[]string"},
		},
		want: "Addr string -> []string",
	}, {
		// A tag change on an unchanged type is named as one, rather than
		// writing the same type on both sides of an arrow and leaving the
		// reader to diff the tags by eye.
		title: "a rewritten tag value is named key by key",
		change: apiFieldChange{
			Name: "Addr", Change: fieldChanged,
			Old: &apiField{Name: "Addr", Type: "string", Tag: `yaml:"addr"`},
			New: &apiField{Name: "Addr", Type: "string", Tag: `yaml:"address"`},
		},
		want: `Addr: yaml tag "addr" -> "address"`,
	}, {
		title: "a tag a field gained is named as added",
		change: apiFieldChange{
			Name: "Children", Change: fieldChanged,
			Old: &apiField{Name: "Children", Type: "[]*StateNode", Tag: `yaml:"children,omitempty"`},
			New: &apiField{Name: "Children", Type: "[]*StateNode", Tag: `json:"children,omitempty" yaml:"children,omitempty"`},
		},
		want: "Children: added json tag",
	}, {
		title: "a tag a field lost is named as removed",
		change: apiFieldChange{
			Name: "Name", Change: fieldChanged,
			Old: &apiField{Name: "Name", Type: "string", Tag: `json:"name" xml:"name"`},
			New: &apiField{Name: "Name", Type: "string", Tag: `json:"name"`},
		},
		want: "Name: removed xml tag",
	}, {
		title: "a tag change beside a type change keeps both shapes whole",
		change: apiFieldChange{
			Name: "Addr", Change: fieldChanged,
			Old: &apiField{Name: "Addr", Type: "string", Tag: `yaml:"addr"`},
			New: &apiField{Name: "Addr", Type: "[]string", Tag: `yaml:"addrs"`},
		},
		want: "Addr string `yaml:\"addr\"` -> []string `yaml:\"addrs\"`",
	}, {
		title: "a tag that does not parse falls back to both shapes",
		change: apiFieldChange{
			Name: "Raw", Change: fieldChanged,
			Old: &apiField{Name: "Raw", Type: "string", Tag: `not a tag`},
			New: &apiField{Name: "Raw", Type: "string", Tag: `json:"raw"`},
		},
		want: "Raw string `not a tag` -> string `json:\"raw\"`",
	}, {
		// An interface method carries its name in the signature it is recorded
		// under, so the name is not written in front of it twice.
		title: "an interface method is not named twice",
		change: apiFieldChange{
			Name: "Put", Change: fieldAdded,
			New: &apiField{Name: "Put", Type: "Put (key string) error"},
		},
		want: "Put (key string) error",
	}, {
		// A field typed after itself is not a name written twice, so it keeps
		// both: the parameter list is what tells a method signature apart.
		title: "a field typed after itself keeps its name",
		change: apiFieldChange{
			Name: "Mode", Change: fieldAdded,
			New: &apiField{Name: "Mode", Type: "Mode", Tag: `yaml:"mode"`},
		},
		want: "Mode Mode `yaml:\"mode\"`",
	}}

	for _, test := range tests {
		if got := fieldText(test.change); got != test.want {
			t.Errorf("%s: fieldText() = %q, want %q", test.title, got, test.want)
		}
	}
}

func TestRenderVerdictNamesTheCommitsBehindAField(t *testing.T) {
	v := sampleVerdict()
	v.CommitAPI = map[string]apiDiff{
		// The commit that added Client carries every field it declares, and
		// the one that reshaped Config carries the field it moved.
		"abc1234": {Added: []apiSymbol{{
			Key: "example.com/x.Client", Name: "Client", Kind: "type", Underlying: "struct",
			Fields: []apiField{{Name: "Name", Type: "string"}},
		}}},
		"def5678": {Types: []apiTypeChange{{
			Key:    "example.com/x.Config",
			Fields: []apiFieldChange{{Name: "Addr", Change: fieldChanged}},
		}}},
	}

	var out bytes.Buffer
	renderVerdict(&out, v, false)
	got := out.String()

	if !strings.Contains(got, "| Change | Package | Type | Field | Commits |") {
		t.Fatalf("renderVerdict() wrote no commits column on the data model:\n%s", got)
	}
	for _, want := range []string{
		"| Name string `json:\"name\"` | [`abc1234`](https://github.com/example/x/commit/abc1234) |",
		"`yaml:\"addr\"` | [`def5678`](https://github.com/example/x/commit/def5678) |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
		}
	}
}

func TestFieldReadsEmbedded(t *testing.T) {
	embedded := apiField{Name: "UnimplementedStorage", Type: "*UnimplementedStorage", Embedded: true}
	if got := fieldReads(embedded); got != "embeds *UnimplementedStorage" {
		t.Errorf("fieldReads(embedded) = %q", got)
	}
	// An embed is read as the type it is declared with, pointer and all: the
	// name splint reaches it by drops the star and says less.
	value := apiField{Name: "Base", Type: "platform.Base", Embedded: true}
	if got := fieldReads(value); got != "embeds platform.Base" {
		t.Errorf("fieldReads(value embed) = %q", got)
	}
	named := apiField{Name: "Addr", Type: "string"}
	if got := fieldReads(named); got != "Addr string" {
		t.Errorf("fieldReads(named) = %q", got)
	}
}
