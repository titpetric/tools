package simpleparser

import (
	"reflect"
	"testing"
)

// TestParseFuncHeaderTypeParams covers what a generic signature records: the
// names its type parameter list binds, and a name with the list taken off.
func TestParseFuncHeaderTypeParams(t *testing.T) {
	tests := []struct {
		header string
		name   string
		params []string
	}{
		{"func Exec[T any](fn func() T) (T, error) {", "Exec", []string{"T"}},
		{"func Keys[K comparable, V any](m map[K]V) []K {", "Keys", []string{"K", "V"}},
		{"func Sum[T, U constraints.Ordered](a T, b U) T {", "Sum", []string{"T", "U"}},
		{"func Serve(addr string) error {", "Serve", nil},
		{"func (r *Runtime) Bind(name string, fn any) error {", "Bind", nil},
	}

	for _, test := range tests {
		decl := parseFuncHeader(test.header)
		if decl == nil {
			t.Errorf("parseFuncHeader(%q) = nil", test.header)
			continue
		}
		if decl.Name != test.name {
			t.Errorf("parseFuncHeader(%q).Name = %q, want %q", test.header, decl.Name, test.name)
		}
		if !reflect.DeepEqual(decl.TypeParams, test.params) {
			t.Errorf("parseFuncHeader(%q).TypeParams = %v, want %v", test.header, decl.TypeParams, test.params)
		}
	}
}

// TestScanFuncWithoutBody pins the extent of a forward declaration: a func
// with no body, a go:linkname target or an assembly stub, ends with its
// signature. Before this, the scanner hunted for a closing brace and read
// every declaration up to the next one as the body, so the const block
// below was never recorded and every name in it stopped resolving.
func TestScanFuncWithoutBody(t *testing.T) {
	src := newSource("x.go", []byte(`package x

//go:linkname alloc runtime.alloc
func alloc(size uintptr) unsafe.Pointer

const (
	kindA kind = iota
	kindB // a trailing comment
)

func used() string { return kindB.String() }
`))

	out := scan(src)

	if len(out.Funcs) != 2 {
		t.Fatalf("Funcs = %d, want the forward declaration and the one after it", len(out.Funcs))
	}
	if got := out.Funcs[0].Name; got != "alloc" {
		t.Errorf("Funcs[0].Name = %q, want alloc", got)
	}
	if len(out.Consts) != 1 {
		t.Fatalf("Consts = %d, want the block the forward declaration used to swallow", len(out.Consts))
	}
	names := out.Consts[0].GetNames()
	want := []string{"kindA", "kindB"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("const names = %v, want %v", names, want)
	}
}
