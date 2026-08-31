package model

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// majorVersion matches the major version a module path ends in, which is not
// part of the name the package is reached by: "example.com/thing/v2" is
// imported as "thing".
//
// It is compiled once. Compiling it inside the loop, which is where it was,
// costs more than everything else the loop does.
var majorVersion = regexp.MustCompile(`/v[0-9]+$`)

// StringSet provides a key based unique string slice.
type StringSet map[string][]string

func NewStringSet() StringSet {
	return make(StringSet)
}

// Keys returns the keys of the set, in order.
func (i *StringSet) Keys() []string {
	keys := make([]string, 0, len(*i))
	for key := range *i {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	return keys
}

func (i *StringSet) Add(key string, lits ...string) {
	data := *i
	if data == nil {
		data = make(StringSet)
	}
	if set, ok := data[key]; ok {
		for _, lit := range lits {
			if slices.Contains(set, lit) {
				return
			}
			set = append(set, lit)
		}

		data[key] = set
		return
	}
	data[key] = lits[:]
	*i = data
}

// Get returns the values under a key, in order.
//
// The copy is the point: sorting the stored slice would reorder the document
// that carries it, so reading a model back from a file would not give the
// model that was written. A reader wanting them in order gets them; the
// document keeps the order it was written in.
func (i StringSet) Get(key string) []string {
	values, ok := i[key]
	if !ok || values == nil {
		return nil
	}

	sorted := make([]string, len(values))
	copy(sorted, values)
	sort.Strings(sorted)
	return sorted
}

// All returns every value of the set, its keys walked in order.
//
// The order is the point. A map is walked at random, and a reader of this is
// deciding something: which of two imports a colliding name was seen under
// first, for one. A run that answered differently each time would be reporting
// the same collision two ways.
func (i StringSet) All() []string {
	result := []string{}
	for _, key := range i.Keys() {
		result = append(result, i[key]...)
	}
	return result
}

// Map returns a map with the short package name as the key
// and the full import path as the value.
func (i StringSet) Map(imports []string) (map[string]string, []error) {
	warnings := []error{}
	warningSeen := map[string]bool{}

	addWarning := func(warning error) {
		msg := warning.Error()
		if _, seen := warningSeen[msg]; !seen {
			warningSeen[msg] = true
			warnings = append(warnings, warning)
		}
	}

	cleanPackageName := func(name string) (string, bool) {
		clean := name
		clean = strings.ReplaceAll(clean, "_", "")
		return clean, name == clean
	}

	result := map[string]string{}
	for _, imported := range imports {
		var short, long string

		// aliased package
		// imported = strings.ReplaceAll(imported, "/go-", "/")
		if strings.Contains(imported, " ") {
			line := strings.Split(imported, " ")
			short, long = line[0], strings.Trim(line[1], `"`)
		} else {
			long = strings.Trim(imported, `"`)
			short = path.Base(long)
		}

		if short == "C" {
			continue
		}

		// trim imported semver link
		if majorVersion.MatchString(long) {
			short = path.Base(majorVersion.ReplaceAllString(long, ""))
		}

		if strings.HasSuffix(short, "_test") {
			clean, ok := cleanPackageName(short[:len(short)-5])
			if !ok {
				addWarning(fmt.Errorf("Alias %s should be %s_test", short, clean))
			}
			continue
		}

		clean, ok := cleanPackageName(short)
		if !ok {
			addWarning(fmt.Errorf("Alias %s should be %s", short, clean))
			continue
		}

		val, ok := result[clean]

		if ok && val != long {
			warning := "Import conflict for %s, "
			// Sort val/long so shorter is left hand side
			if len(val) < len(long) {
				warning += val + " != " + long
			} else {
				warning += long + " != " + val
			}
			addWarning(fmt.Errorf(warning, short))
		}

		result[clean] = long
	}

	return result, warnings
}
