package simpleparser

import (
	"reflect"
	"testing"
)

// TestExits counts the ways each function leaves: a return, a call that ends
// the program, and a body running off its end. A literal's return is its own.
func TestExits(t *testing.T) {
	t.Parallel()

	out := scan(newSource("x.go", []byte(`package x

import (
	"log"
	"os"
)

func Empty() {}

func Inline() int { return 1 }

func Falls(v int) {
	if v > 0 {
		println(v)
	}
}

func Returns(v int) error {
	if v > 0 {
		return nil
	}
	return os.ErrNotExist
}

func Mixed(v int) int {
	switch v {
	case 0:
		panic("zero")
	case 1:
		os.Exit(1)
	case 2:
		log.Fatalf("two")
	}
	return v
}

func Closure() func() int {
	f := func() int {
		return 1
	}
	return func() int {
		return f()
	}
}

func Multiline() *os.File {
	return &os.File{
	}
}
`)))

	got := map[string]int{}
	for _, decl := range out.Funcs {
		got[decl.Name] = decl.Complexity.Exits
	}
	want := map[string]int{
		"Empty": 1, "Inline": 1, "Falls": 1, "Returns": 2,
		"Mixed": 4, "Closure": 1, "Multiline": 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Exits = %v, want %v", got, want)
	}
}
