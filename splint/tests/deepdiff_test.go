package tests_test

import (
	"testing"

	"github.com/titpetric/tools/splint/tests"
)

// TestDiffFindsWhatIsNotEnumerated is the point of the comparison: a key
// nobody wrote a check for is still compared, and a key only one side carries
// is a difference rather than a silence.
func TestDiffFindsWhatIsNotEnumerated(t *testing.T) {
	left := map[string]any{
		"Name":    "Open",
		"Line":    12,
		"Nested":  map[string]any{"A": 1, "B": 2},
		"List":    []any{"x", "y"},
		"OnlyOne": "here",
	}
	right := map[string]any{
		"Name":   "Open",
		"Line":   13,
		"Nested": map[string]any{"A": 1, "B": 3},
		"List":   []any{"x", "z"},
		"Added":  "new",
	}

	got := map[string]string{}
	for _, difference := range tests.Diff(left, right, nil) {
		got[difference.Path] = difference.Left + " != " + difference.Right
	}

	want := map[string]string{
		"Line":     "12 != 13",
		"Nested.B": "2 != 3",
		"List[1]":  "y != z",
		"OnlyOne":  "here != <absent>",
		"Added":    "<absent> != new",
	}
	if len(got) != len(want) {
		t.Fatalf("Diff() = %v, want %v", got, want)
	}
	for path, difference := range want {
		if got[path] != difference {
			t.Errorf("Diff()[%q] = %q, want %q", path, got[path], difference)
		}
	}
}

func TestDiffReportsALengthChange(t *testing.T) {
	got := tests.Diff(map[string]any{"L": []any{1, 2, 3}}, map[string]any{"L": []any{1, 2}}, nil)

	if len(got) != 1 || got[0].Path != "L.length" {
		t.Fatalf("Diff() = %v, want one length difference", got)
	}
	if got[0].Left != "3" || got[0].Right != "2" {
		t.Errorf("Diff() = %v", got[0])
	}
}

// TestDiffNormalise covers how a difference that is known and explained is
// taken out of the comparison without taking the field out of it.
func TestDiffNormalise(t *testing.T) {
	left := map[string]any{"Source": "a   b", "Other": "a   b"}
	right := map[string]any{"Source": "a b", "Other": "a b"}

	normalise := func(path string, a, b any) (any, any) {
		if path == "Source" {
			return "", ""
		}
		return a, b
	}

	got := tests.Diff(left, right, normalise)
	if len(got) != 1 || got[0].Path != "Other" {
		t.Errorf("Diff() = %v, want the unnormalised field alone", got)
	}
}

func TestDiffOnAKeyedList(t *testing.T) {
	left := map[string]any{"Funcs": tests.KeyedList{
		"x.go.Open":  map[string]any{"Line": 1},
		"x.go.Close": map[string]any{"Line": 9},
	}}
	right := map[string]any{"Funcs": tests.KeyedList{
		"x.go.Open": map[string]any{"Line": 2},
		"x.go.Read": map[string]any{"Line": 4},
	}}

	got := map[string]string{}
	for _, difference := range tests.Diff(left, right, nil) {
		got[difference.Path] = difference.Left + " != " + difference.Right
	}

	// A list keyed by identity reports one difference per symbol rather than
	// shifting every element after the one that moved.
	want := map[string]string{
		"Funcs[x.go.Open].Line": "1 != 2",
		"Funcs[x.go.Close]":     "present != <absent>",
		"Funcs[x.go.Read]":      "<absent> != present",
	}
	if len(got) != len(want) {
		t.Fatalf("Diff() = %v, want %v", got, want)
	}
	for path, difference := range want {
		if got[path] != difference {
			t.Errorf("Diff()[%q] = %q, want %q", path, got[path], difference)
		}
	}
}

func TestField(t *testing.T) {
	tests_ := map[string]string{
		"Funcs[x.go.Open].Complexity.Cognitive": "Funcs[].Complexity.Cognitive",
		"Fields[2].Tag":                         "Fields[].Tag",
		"Types[a.b].Fields[0].Name":             "Types[].Fields[].Name",
		"Doc":                                   "Doc",
		"List.length":                           "List.length",
	}

	for path, want := range tests_ {
		if got := tests.Field(path); got != want {
			t.Errorf("Field(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestScalars(t *testing.T) {
	value := map[string]any{
		"a": 1,
		"b": []any{1, 2, 3},
		"c": map[string]any{"d": "x", "e": nil},
		"f": tests.KeyedList{"g": map[string]any{"h": true}},
	}

	// Six leaves: a, three of b, d, and h. A nil is absent rather than a value.
	if got := tests.Scalars(value); got != 6 {
		t.Errorf("Scalars() = %d, want 6", got)
	}
}
