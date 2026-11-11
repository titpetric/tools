package simpleparser

import (
	"reflect"
	"testing"
)

// TestBlocks counts the braces each body opens: a statement block, a function
// literal, a composite or struct literal. A brace in the signature or inside a
// string belongs to neither.
func TestBlocks(t *testing.T) {
	t.Parallel()

	out := scan(newSource("x.go", []byte(`package x

func Empty() {}

func Inline() int { return 1 }

func Assembly(x int) int

func Cond(v int) error {
	if v > 0 {
		return nil
	}
	return nil
}

func Chain(v int) int {
	if v > 0 {
		return 1
	} else if v < 0 {
		return -1
	} else {
		return 0
	}
}

func Loop(vs []int) int {
	total := 0
	for _, v := range vs {
		if v > 0 {
			total += v
		}
	}
	return total
}

func Switch(v int) string {
	switch v {
	case 0:
		return "zero"
	default:
		return "other"
	}
}

func Literal() map[string]int {
	return map[string]int{"a": 1}
}

func Closure() func() int {
	f := func() int {
		return 1
	}
	return f
}

func Signature(v interface{}, done chan struct{}) struct{} {
	_ = v
	close(done)
	return struct{}{}
}

func Braced() string {
	return "{"
}
`)))

	got := map[string]int{}
	for _, decl := range out.Funcs {
		got[decl.Name] = decl.Complexity.Blocks
	}
	want := map[string]int{
		"Empty": 0, "Inline": 0, "Assembly": 0, "Cond": 1, "Chain": 3,
		"Loop": 2, "Switch": 1, "Literal": 1, "Closure": 1, "Signature": 2,
		"Braced": 0,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Blocks = %v, want %v", got, want)
	}
}
