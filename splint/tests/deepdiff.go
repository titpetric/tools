// Package tests holds the harness that compares the two parsers.
//
// It is a package rather than a test file so the comparison can be read and
// reused: what it does is walk two documents and say where they differ, which
// is worth having outside the test that runs it.
package tests

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// KeyedList is a list rendered as a map from identity to element, which is how
// a comparison lines two documents up by what a thing is rather than by where
// it sits.
//
// It is a type of its own so the walk can tell it from a map the model
// declares: an element of a keyed list is named in brackets, "Funcs[Open]", so
// the summary can drop the key and count every element under the field.
type KeyedList map[string]any

// Diff walks two values and returns every place they disagree.
//
// It is a deep comparison of the encoded form rather than a list of fields
// somebody remembered to check: a key that only one side carries is a
// difference, and so is a key added to the model tomorrow. Nothing is covered
// by being enumerated here, because nothing is enumerated here.
//
// The normalise function is given a path and the two values at it, and returns
// what should be compared instead. It is how a difference that is known and
// explained, such as column alignment inside a source listing, is taken out of
// the comparison without taking the field out of it.
func Diff(left, right any, normalise func(path string, a, b any) (any, any)) []Difference {
	var found []Difference
	walk("", encode(left), encode(right), normalise, &found)
	sort.Slice(found, func(i, j int) bool { return found[i].Path < found[j].Path })
	return found
}

// encode renders a value as the generic tree its JSON form describes, which is
// what makes the walk indifferent to the types behind it.
//
// A tree that is already generic is passed through: encoding it again would
// turn a KeyedList back into the map it is made of, and the walk would lose
// the one thing that lines two documents up by identity.
func encode(value any) any {
	switch value.(type) {
	case KeyedList, map[string]any, []any, string, float64, bool, nil:
		return value
	}

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("unencodable: %v", err)
	}

	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return fmt.Sprintf("unreadable: %v", err)
	}
	return out
}

// Scalars is how many leaves a tree has, which is what a difference rate is
// counted against: a rate over declarations says nothing about a field only
// some declarations carry.
func Scalars(value any) int {
	switch v := value.(type) {
	case KeyedList:
		total := 0
		for _, inner := range v {
			total += Scalars(inner)
		}
		return total
	case map[string]any:
		total := 0
		for _, inner := range v {
			total += Scalars(inner)
		}
		return total
	case []any:
		total := 0
		for _, inner := range v {
			total += Scalars(inner)
		}
		return total
	case nil:
		return 0
	}
	return 1
}

// walk compares two encoded values and records where they part company.
func walk(path string, left, right any, normalise func(string, any, any) (any, any), found *[]Difference) {
	if normalise != nil {
		left, right = normalise(path, left, right)
	}

	switch a := left.(type) {
	case KeyedList:
		b, ok := right.(KeyedList)
		if !ok {
			record(path, left, right, found)
			return
		}
		for _, key := range unionKeys(a, b) {
			element := fmt.Sprintf("%s[%s]", path, key)
			switch {
			case a[key] == nil:
				record(element, "<absent>", "present", found)
			case b[key] == nil:
				record(element, "present", "<absent>", found)
			default:
				walk(element, a[key], b[key], normalise, found)
			}
		}

	case map[string]any:
		b, ok := right.(map[string]any)
		if !ok {
			record(path, left, right, found)
			return
		}
		for _, key := range unionKeys(a, b) {
			walk(join(path, key), a[key], b[key], normalise, found)
		}

	case []any:
		b, ok := right.([]any)
		if !ok {
			record(path, left, right, found)
			return
		}
		if len(a) != len(b) {
			record(path+".length", len(a), len(b), found)
		}
		for i := 0; i < len(a) && i < len(b); i++ {
			walk(fmt.Sprintf("%s[%d]", path, i), a[i], b[i], normalise, found)
		}

	default:
		if !equal(left, right) {
			record(path, left, right, found)
		}
	}
}

// record adds one difference.
func record(path string, left, right any, found *[]Difference) {
	*found = append(*found, Difference{
		Path:  path,
		Left:  render(left),
		Right: render(right),
	})
}

// equal compares two scalars, treating a missing value and a zero one as
// different: a field that is absent on one side is a difference worth seeing.
func equal(left, right any) bool {
	return render(left) == render(right)
}

// render writes a scalar for comparison and for the report.
func render(value any) string {
	switch v := value.(type) {
	case nil:
		return "<absent>"
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%v", v)
	}

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}

// unionKeys returns every key either map carries, in order, so a key only one
// side has is walked rather than skipped.
func unionKeys[M ~map[string]any](left, right M) []string {
	seen := make(map[string]bool, len(left)+len(right))
	keys := make([]string, 0, len(left)+len(right))

	for _, m := range []M{left, right} {
		for key := range m {
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}

	sort.Strings(keys)
	return keys
}

// join adds one element to a path.
func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// Field is a path with its array indices removed, so every element of a list
// is counted under the field it belongs to rather than under its position:
// "Fields[2].Tag" and "Fields[7].Tag" are both "Fields[].Tag".
func Field(path string) string {
	var out strings.Builder

	for i := 0; i < len(path); i++ {
		if path[i] != '[' {
			out.WriteByte(path[i])
			continue
		}
		close := strings.IndexByte(path[i:], ']')
		if close < 0 {
			out.WriteByte(path[i])
			continue
		}
		out.WriteString("[]")
		i += close
	}

	return out.String()
}

// clip shortens a value to something a report line holds.
func clip(value string) string {
	value = strings.ReplaceAll(value, "\n", "\\n")
	if len(value) > 90 {
		return value[:90] + "[...]"
	}
	return value
}
