package model

import "testing"

func TestDeclarationSymbol(t *testing.T) {
	tests := []struct {
		decl Declaration
		want string
	}{
		{decl: Declaration{Name: "Open"}, want: "Open"},
		{decl: Declaration{Name: "Close", Receiver: "*Client"}, want: "Client.Close"},
		{decl: Declaration{Name: "Close", Receiver: "Client"}, want: "Client.Close"},
		// A const block declares its names as a list and carries no single one.
		{decl: Declaration{Names: []string{"KindA", "KindB"}}, want: "KindA"},
		{decl: Declaration{}, want: ""},
	}

	for _, test := range tests {
		if got := test.decl.Symbol(); got != test.want {
			t.Errorf("Symbol() of %#v = %q, want %q", test.decl, got, test.want)
		}
	}
}

func TestDeclarationPosition(t *testing.T) {
	decl := Declaration{File: "handler.go", Line: 42}

	tests := []struct {
		path string
		want string
	}{
		// The root package records "." and adds no directory.
		{path: ".", want: "handler.go"},
		{path: "./frontend", want: "frontend/handler.go"},
		{path: "./frontend/view", want: "frontend/view/handler.go"},
	}

	for _, test := range tests {
		got := decl.Position(Package{Package: "frontend", Path: test.path})
		if got.File != test.want {
			t.Errorf("Position(%q).File = %q, want %q", test.path, got.File, test.want)
		}
		if got.Line != 42 || got.Package != "frontend" {
			t.Errorf("Position(%q) = %#v", test.path, got)
		}
	}
}
