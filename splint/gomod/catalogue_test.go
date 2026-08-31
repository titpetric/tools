package gomod_test

import (
	"reflect"
	"testing"

	"github.com/titpetric/tools/splint/gomod"
	"github.com/titpetric/tools/splint/model"
)

// document is a tree of two modules, one requiring the other's dependency, so
// the catalogue has something to merge.
func document() *model.DocumentRoot {
	return &model.DocumentRoot{
		Modules: []*model.Module{
			{
				Path: "example.com/main",
				Requires: []model.Require{
					{Path: "example.com/x", Version: "v1.2.0"},
					{Path: "example.com/x/inner", Version: "v0.1.0"},
					{Path: "example.com/deep", Version: "v0.4.0", Indirect: true},
				},
				Replaces: []model.Replace{
					{Path: "example.com/x", NewPath: "../x"},
				},
			},
			{
				Path: "example.com/main/tool",
				Requires: []model.Require{
					// The same module, required directly here and indirectly
					// above. The direct requirement is the one that decides.
					{Path: "example.com/deep", Version: "v0.4.0"},
				},
			},
		},
	}
}

func TestCatalogue_Owner(t *testing.T) {
	c := gomod.NewCatalogue(document())

	tests := map[string]string{
		// The longest match wins, so a submodule required on its own is not
		// attributed to the module it sits under.
		"example.com/x":              "example.com/x",
		"example.com/x/sub":          "example.com/x",
		"example.com/x/inner":        "example.com/x/inner",
		"example.com/x/inner/deeper": "example.com/x/inner",
		// A path boundary, not a prefix: xylophone is not x.
		"example.com/xylophone": "",
		"example.com/nothing":   "",
	}

	for path, want := range tests {
		require, ok := c.Owner(path)
		switch {
		case want == "" && ok:
			t.Errorf("Owner(%q) = %q, want nothing", path, require.Path)
		case want != "" && require.Path != want:
			t.Errorf("Owner(%q) = %q, want %q", path, require.Path, want)
		}
	}
}

// TestCatalogue_Requires covers the merge: a module required by two of the
// modules read is one dependency, and the direct requirement wins.
func TestCatalogue_Requires(t *testing.T) {
	c := gomod.NewCatalogue(document())

	got := c.Requires()
	if len(got) != 3 {
		t.Fatalf("Requires() = %d, want 3: %#v", len(got), got)
	}

	for _, require := range got {
		if require.Path == "example.com/deep" && require.Indirect {
			t.Error("Requires() kept the indirect half of a module required both ways")
		}
	}
}

func TestCatalogue_Replaced(t *testing.T) {
	c := gomod.NewCatalogue(document())

	if replace, ok := c.Replaced("example.com/x"); !ok || replace.NewPath != "../x" {
		t.Errorf("Replaced() = %#v, %v", replace, ok)
	}
	if _, ok := c.Replaced("example.com/deep"); ok {
		t.Error("Replaced() invented a directive")
	}
	if got := c.Replaces(); len(got) != 1 {
		t.Errorf("Replaces() = %#v, want one", got)
	}
}

// TestCatalogue_Owns covers how code of the tree is told from a dependency.
func TestCatalogue_Owns(t *testing.T) {
	c := gomod.NewCatalogue(document())

	for _, path := range []string{"example.com/main", "example.com/main/inner", "example.com/main/tool"} {
		if !c.Owns(path) {
			t.Errorf("Owns(%q) = false", path)
		}
	}
	if c.Owns("example.com/x") {
		t.Error("Owns() claimed a dependency")
	}
}

// TestCatalogue_Sums covers a version two modules of the tree both record.
// They resolve against one module cache, so it is one version, and the hash of
// the source on either side says the build downloads it.
func TestCatalogue_Sums(t *testing.T) {
	doc := document()
	doc.Modules[0].Sums = []model.Sum{
		{Path: "example.com/x", Version: "v1.2.0"},
		{Path: "example.com/deep", Version: "v0.4.0", Zip: true},
	}
	doc.Modules[1].Sums = []model.Sum{
		{Path: "example.com/x", Version: "v1.2.0", Zip: true},
	}

	got := gomod.NewCatalogue(doc).Sums()

	want := []model.Sum{
		{Path: "example.com/deep", Version: "v0.4.0", Zip: true},
		{Path: "example.com/x", Version: "v1.2.0", Zip: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Sums() = %#v, want %#v", got, want)
	}
}

func TestCatalogueOnNothing(t *testing.T) {
	c := gomod.NewCatalogue(nil)

	if _, ok := c.Owner("example.com/x"); ok {
		t.Error("Owner() answered for an empty catalogue")
	}
	if len(c.Requires()) != 0 || len(c.Replaces()) != 0 || len(c.Modules()) != 0 {
		t.Error("an empty catalogue holds something")
	}
}

// TestBase covers the suffix that makes two majors of one module read as two
// modules, which is what tells them apart from two unrelated ones.
func TestBase(t *testing.T) {
	tests := map[string]string{
		"example.com/x":     "example.com/x",
		"example.com/x/v2":  "example.com/x",
		"example.com/x/v12": "example.com/x",
		// Not a major version: a directory that happens to start with a v.
		"example.com/x/view":   "example.com/x/view",
		"example.com/x/v":      "example.com/x/v",
		"example.com/x/v2beta": "example.com/x/v2beta",
		"single":               "single",
	}

	for path, want := range tests {
		if got := gomod.Base(path); got != want {
			t.Errorf("Base(%q) = %q, want %q", path, got, want)
		}
	}
}
