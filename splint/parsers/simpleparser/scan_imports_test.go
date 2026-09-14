package simpleparser

import (
	"reflect"
	"testing"
)

// TestScanImports covers what the model records an import as: the path in
// quotes, with an alias in front of it only when the alias says something the
// path does not.
func TestScanImports(t *testing.T) {
	src := newSource("x.go", []byte(`package p

import (
	"fmt"
	rand "math/rand/v2"
	strings "strings"
	_ "embed"
	. "errors"
	alias "example.com/other" // a comment
)
`))

	file := &file{aliases: map[string]bool{}}
	end := file.scanImports(src, 2)
	if end != 9 {
		t.Errorf("scanImports() ended at %d, want the closing paren at 9", end)
	}

	want := []string{
		`"fmt"`,
		// An alias that differs from the base name is recorded with it.
		`rand "math/rand/v2"`,
		// One that repeats the base name says nothing and is dropped.
		`"strings"`,
		// A blank import is reached by no name, and the underscore is what
		// records that: it is an import for its side effect alone.
		`_ "embed"`,
		// A dot import puts the package's names in this file's scope, which
		// the literal records even though no name reaches it.
		`. "errors"`,
		`alias "example.com/other"`,
	}
	if !reflect.DeepEqual(file.Imports, want) {
		t.Errorf("Imports = %#v, want %#v", file.Imports, want)
	}

	// A blank or dot import is reached by no name, so neither is a name a
	// reference can be recognised under.
	for _, name := range []string{"fmt", "rand", "strings", "alias"} {
		if !file.aliases[name] {
			t.Errorf("aliases missing %q: %#v", name, file.aliases)
		}
	}
	if file.aliases["embed"] || file.aliases["errors"] {
		t.Errorf("aliases holds a blank or dot import: %#v", file.aliases)
	}
}

func TestScanImportsSingle(t *testing.T) {
	src := newSource("x.go", []byte("package p\n\nimport \"fmt\"\n"))

	file := &file{aliases: map[string]bool{}}
	if end := file.scanImports(src, 2); end != 2 {
		t.Errorf("scanImports() ended at %d, want 2", end)
	}
	if !reflect.DeepEqual(file.Imports, []string{`"fmt"`}) {
		t.Errorf("Imports = %#v", file.Imports)
	}
}

func TestSplitImport(t *testing.T) {
	tests := []struct {
		entry  string
		alias  string
		quoted string
	}{
		{entry: `"fmt"`, alias: "", quoted: `"fmt"`},
		{entry: `rand "math/rand/v2"`, alias: "rand", quoted: `"math/rand/v2"`},
		{entry: `_ "embed"`, alias: "_", quoted: `"embed"`},
		{entry: `"fmt" // why`, alias: "", quoted: `"fmt"`},
		{entry: "", alias: "", quoted: ""},
		{entry: "nothing quoted", alias: "", quoted: ""},
	}

	for _, test := range tests {
		alias, quoted := splitImport(test.entry)
		if alias != test.alias || quoted != test.quoted {
			t.Errorf("splitImport(%q) = %q, %q, want %q, %q", test.entry, alias, quoted, test.alias, test.quoted)
		}
	}
}
