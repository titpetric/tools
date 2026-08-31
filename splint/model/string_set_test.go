package model

import (
	"strings"
	"testing"
)

func TestStringSet(t *testing.T) {
	s := NewStringSet()
	s.Add("test.go", `"net/http"`, `"github.com/huandu/go-clone"`, `modelName "github.com/package/model"`)
	imports := s.All()
	m, errs := s.Map(imports)
	t.Log(m, errs)
}

// TestStringSetGetDoesNotReorderTheSet covers what a read must not do to what
// it read: sorting the stored slice would mean a document loaded from a file
// is not the document that was written to it.
func TestStringSetGetDoesNotReorderTheSet(t *testing.T) {
	set := NewStringSet()
	set.Add("x.go", `"fmt"`, `"strings"`, `"example.com/a"`)

	got := set.Get("x.go")
	want := []string{`"example.com/a"`, `"fmt"`, `"strings"`}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Get() = %v, want %v", got, want)
	}

	stored := []string{`"fmt"`, `"strings"`, `"example.com/a"`}
	if strings.Join(set["x.go"], ",") != strings.Join(stored, ",") {
		t.Errorf("Get() reordered the set to %v, want %v", set["x.go"], stored)
	}

	if set.Get("missing") != nil {
		t.Error("Get() invented a value for a key that has none")
	}
}

// TestStringSetKeysDoesNotReorderTheSet covers the same for the key listing,
// which used to sort every value slice on its way past.
func TestStringSetKeysDoesNotReorderTheSet(t *testing.T) {
	set := NewStringSet()
	set.Add("b.go", `"z"`, `"a"`)
	set.Add("a.go", `"y"`)

	if keys := set.Keys(); strings.Join(keys, ",") != "a.go,b.go" {
		t.Errorf("Keys() = %v", keys)
	}
	if strings.Join(set["b.go"], ",") != `"z","a"` {
		t.Errorf("Keys() reordered a value slice to %v", set["b.go"])
	}
}
