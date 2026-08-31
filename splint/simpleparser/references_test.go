package simpleparser

import (
	"reflect"
	"testing"
)

func TestSelectors(t *testing.T) {
	tests := []struct {
		line string
		want []selector
	}{
		{line: "http.Error(w, msg, http.StatusNotFound)", want: []selector{
			{pkg: "http", symbol: "Error"},
			{pkg: "http", symbol: "StatusNotFound"},
		}},
		// A chained selector reaches one package through one symbol; what
		// follows belongs to whatever that returned.
		{line: "a.B.C()", want: []selector{{pkg: "a", symbol: "B"}}},
		// A number is not a selector, and neither is a call result.
		{line: "x := 1.5", want: nil},
		{line: "f().X", want: nil},
		{line: "no selector here", want: nil},
	}

	for _, test := range tests {
		if got := selectors(test.line); !reflect.DeepEqual(got, test.want) {
			t.Errorf("selectors(%q) = %#v, want %#v", test.line, got, test.want)
		}
	}
}

// TestFileReferences covers the two things that decide a reference: the name
// has to be one the file imports, and the symbol has to be exported.
func TestFileReferences(t *testing.T) {
	src := newSource("x.go", []byte(`package p

func f() {
	fmt.Println(os.Args)
	local.thing()
	fmt.unexported()
}
`))

	file := &file{aliases: map[string]bool{"fmt": true, "os": true}}
	got := file.references(src, 3, 0, 6, nil)

	want := map[string][]string{"fmt": {"Println"}, "os": {"Args"}}
	if !reflect.DeepEqual(map[string][]string(got), want) {
		t.Errorf("references() = %#v, want %#v", got, want)
	}
}

// TestFileReferencesSkipsShadowedNames covers a parameter named after a
// package, which is the package's name everywhere but inside that function.
func TestFileReferencesSkipsShadowedNames(t *testing.T) {
	src := newSource("x.go", []byte(`package p

func Run(config config.Config) {
	fmt.Println(config.Env)
}
`))

	file := &file{aliases: map[string]bool{"config": true, "fmt": true}}
	got := file.references(src, 3, 0, 4, []string{"config"})

	want := map[string][]string{"fmt": {"Println"}}
	if !reflect.DeepEqual(map[string][]string(got), want) {
		t.Errorf("references() = %#v, want %#v", got, want)
	}
}

func TestExported(t *testing.T) {
	tests := map[string]bool{"Name": true, "name": false, "": false, "_x": false}

	for name, want := range tests {
		if got := exported(name); got != want {
			t.Errorf("exported(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestTrimTypeParams(t *testing.T) {
	tests := map[string]string{
		"List":          "List",
		"List[T any]":   "List",
		"Map[K, V any]": "Map",
	}

	for name, want := range tests {
		if got := trimTypeParams(name); got != want {
			t.Errorf("trimTypeParams(%q) = %q, want %q", name, got, want)
		}
	}
}
