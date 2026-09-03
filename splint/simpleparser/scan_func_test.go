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
