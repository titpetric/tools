package simpleparser

import (
	"reflect"
	"testing"
)

func TestUses(t *testing.T) {
	src := newSource("x.go", []byte(`package p

func f(w io.Writer) {
	fmt.Fprintln(w, os.Args)
}
`))

	want := []string{"fmt", "io", "os"}
	if got := uses(src); !reflect.DeepEqual(got, want) {
		t.Errorf("uses() = %#v, want %#v", got, want)
	}
}

// TestUsesReadsAVariadicParameterType covers the name an import is kept by
// when the only thing reaching it is a "...pkg.Type" parameter.
//
// The set answers whether removing an import breaks the file, and the fixer
// takes a name missing from it as an import nothing reaches. A signature is
// the one place a package can be reached from without the body naming it, so
// a miss here is an import deleted out of a file that compiles.
func TestUsesReadsAVariadicParameterType(t *testing.T) {
	src := newSource("x.go", []byte(`package p

func New(baseFS fs.FS, opts ...vuego.LoadOption) *Service {
	return nil
}
`))

	want := []string{"fs", "vuego"}
	if got := uses(src); !reflect.DeepEqual(got, want) {
		t.Errorf("uses() = %#v, want %#v", got, want)
	}
}

// TestUsesIgnoresASpreadArgument covers the same token in the other position,
// where it follows a value instead of introducing a type and reaches nothing
// of its own.
func TestUsesIgnoresASpreadArgument(t *testing.T) {
	src := newSource("x.go", []byte(`package p

func f(args []any) {
	fmt.Println(args...)
}
`))

	want := []string{"fmt"}
	if got := uses(src); !reflect.DeepEqual(got, want) {
		t.Errorf("uses() = %#v, want %#v", got, want)
	}
}
