package model

import "testing"

func TestFileBase(t *testing.T) {
	tests := map[string]string{
		"handler.go":       "handler",
		"handler_test.go":  "handler",
		"handler_linux.go": "handler_linux",
		"x.go":             "x",
	}

	for name, want := range tests {
		if got := (File{Name: name}).Base(); got != want {
			t.Errorf("File{%q}.Base() = %q, want %q", name, got, want)
		}
	}
}

func TestFileList(t *testing.T) {
	files := FileList{
		{Name: "a.go", Lines: 10},
		{Name: "a_test.go", Lines: 20, Test: true},
		{Name: "b.go", Lines: 5, Generated: true},
	}

	if got := files.Lines(); got != 35 {
		t.Errorf("Lines() = %d, want 35", got)
	}

	if file, ok := files.Find("a_test.go"); !ok || !file.Test {
		t.Errorf("Find(a_test.go) = %#v, %v", file, ok)
	}
	if _, ok := files.Find("missing.go"); ok {
		t.Error("Find() invented a file")
	}

	// A filter keeps what it accepts and nothing else, which is how the two
	// halves of a package are told apart.
	if got := files.Filter(func(f File) bool { return !f.Test }); len(got) != 2 {
		t.Errorf("Filter() kept %d files, want 2", len(got))
	}
	if got := files.Filter(func(f File) bool { return f.Generated }); len(got) != 1 || got[0].Name != "b.go" {
		t.Errorf("Filter() = %#v", got)
	}
}
