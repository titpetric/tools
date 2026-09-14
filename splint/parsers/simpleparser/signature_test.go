package simpleparser

import (
	"reflect"
	"testing"
)

func TestSplitTop(t *testing.T) {
	tests := []struct {
		list string
		want []string
	}{
		{list: "a, b", want: []string{"a", "b"}},
		// A comma inside a type does not separate two parameters.
		{list: "fn func(a, b int) error", want: []string{"fn func(a, b int) error"}},
		{list: "m map[string]int, s []byte", want: []string{"m map[string]int", "s []byte"}},
		{list: "v struct{ A, B int }, n int", want: []string{"v struct{ A, B int }", "n int"}},
		{list: "", want: nil},
	}

	for _, test := range tests {
		if got := splitTop(test.list, ','); !reflect.DeepEqual(got, test.want) {
			t.Errorf("splitTop(%q) = %#v, want %#v", test.list, got, test.want)
		}
	}
}

func TestNamedList(t *testing.T) {
	tests := map[string]bool{
		// The last item decides for the whole list, which is what Go requires
		// of it.
		"a, b int":                   true,
		"name string":                true,
		"ctx context.Context, n int": true,
		"fn func(a, b int) error":    true,
		"Trace, bool":                false,
		"int, error":                 false,
		"string":                     false,
		"chan int":                   false,
		"map[string]int":             false,
		"func(a, b int) error":       false,
		"...string":                  false,
		"names ...string":            true,
	}

	for list, want := range tests {
		if got := namedList(splitTop(list, ',')); got != want {
			t.Errorf("namedList(%q) = %v, want %v", list, got, want)
		}
	}
}

func TestParamGroups(t *testing.T) {
	tests := []struct {
		list string
		want []paramGroup
	}{
		{list: "a, b int", want: []paramGroup{{Names: []string{"a", "b"}, Type: "int"}}},
		{list: "Trace, bool", want: []paramGroup{{Type: "Trace"}, {Type: "bool"}}},
		{list: "ctx context.Context, n int", want: []paramGroup{
			{Names: []string{"ctx"}, Type: "context.Context"},
			{Names: []string{"n"}, Type: "int"},
		}},
		{list: "", want: nil},
	}

	for _, test := range tests {
		if got := paramGroups(test.list); !reflect.DeepEqual(got, test.want) {
			t.Errorf("paramGroups(%q) = %#v, want %#v", test.list, got, test.want)
		}
	}
}

// TestSignature covers the shape the model records, which writes a parameter
// with the names in front of it and a result with the type alone.
func TestSignature(t *testing.T) {
	tests := []struct {
		name    string
		params  string
		results string
		want    string
	}{
		{name: "Open", params: "name string", results: "error", want: "Open (name string) error"},
		{name: "Open", params: "a, b int", results: "", want: "Open (a,b int)"},
		{name: "Trace", params: "id string", results: "(Trace, bool)", want: "Trace (id string) (Trace, bool)"},
		// A named result is written as its type, and a group sharing one type
		// is one result: "(name, host string)" is a single field to the
		// collector, so it prints one type and not two.
		{name: "groupKey", params: "trace Trace", results: "(name, host string)", want: "groupKey (trace Trace) string"},
		{name: "Read", params: "", results: "(n int, err error)", want: "Read () (int, error)"},
		{name: "Close", params: "", results: "", want: "Close ()"},
	}

	for _, test := range tests {
		got := signature(test.name, paramGroups(test.params), resultGroups(test.results))
		if got != test.want {
			t.Errorf("signature(%q, %q, %q) = %q, want %q", test.name, test.params, test.results, got, test.want)
		}
	}
}

func TestGroupTypes(t *testing.T) {
	// The collector reads one type per field and drops a repeat, so a function
	// taking two strings reports one.
	got := groupTypes(paramGroups("a string, b string, n int"))
	want := []string{"string", "int"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("groupTypes() = %#v, want %#v", got, want)
	}
}

func TestSymbolType(t *testing.T) {
	tests := map[string]string{
		"int":              "int",
		"*Client":          "*Client",
		"interface{}":      "any",
		"interface{ A() }": "any",
		// An array is written as a slice of its element, which is what the
		// collector prints.
		"[16]byte":       "[]byte",
		"[]byte":         "[]byte",
		"[N]int":         "[]int",
		"map[string]int": "map[string]int",
	}

	for typ, want := range tests {
		if got := symbolType(typ); got != want {
			t.Errorf("symbolType(%q) = %q, want %q", typ, got, want)
		}
	}
}

func TestIsIdentifier(t *testing.T) {
	tests := map[string]bool{
		"name":   true,
		"_":      true,
		"n1":     true,
		"1n":     false,
		"":       false,
		"[]byte": false,
		"int":    false,
		"struct": false,
		"Client": true,
	}

	for word, want := range tests {
		if got := isIdentifier(word); got != want {
			t.Errorf("isIdentifier(%q) = %v, want %v", word, got, want)
		}
	}
}
