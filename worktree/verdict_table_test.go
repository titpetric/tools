package main

import (
	"testing"
)

func TestShortPackage(t *testing.T) {
	tests := []struct {
		module, pkg string
		want        string
	}{
		// The package at the root of the module is the module root, which is
		// what the leading slash names.
		{module: "example.com/x", pkg: "example.com/x", want: "/"},
		{module: "github.com/titpetric/vuego-cli", pkg: "github.com/titpetric/vuego-cli/config", want: "/config"},
		// A package below the module keeps its whole path under the root, so
		// two packages sharing a name stay apart.
		{module: "example.com/x", pkg: "example.com/x/commands/host", want: "/commands/host"},
		{module: "example.com/x", pkg: "example.com/x/frontend/model", want: "/frontend/model"},
		// A package outside the module, which the model should not hold, is
		// named as it stands rather than mangled.
		{module: "example.com/x", pkg: "example.com/other", want: "example.com/other"},
	}

	for _, test := range tests {
		if got := shortPackage(test.module, test.pkg); got != test.want {
			t.Errorf("shortPackage(%q, %q) = %q, want %q", test.module, test.pkg, got, test.want)
		}
	}
}
