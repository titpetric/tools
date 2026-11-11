package mdfmt

import (
	"slices"
	"testing"
)

func TestCodeSpans(t *testing.T) {
	for name, one := range map[string]struct {
		source string
		want   []string
	}{
		"none":           {"# T\n\nPlain prose.\n", nil},
		"one":            {"A `name` here.\n", []string{"name"}},
		"two in order":   {"A `first` and a `second`.\n", []string{"first", "second"}},
		"holding a tick": {"Names wear `` `Thing::run` `` backticks.\n", []string{"`Thing::run`"}},
		"a fence is not": {"```go\nfunc x() {}\n```\n", nil},
		"inside a table": {"| a | b |\n|---|---|\n| `x` | y |\n", []string{"x"}},
	} {
		t.Run(name, func(t *testing.T) {
			if got := codeSpans([]byte(one.source)); !slices.Equal(got, one.want) {
				t.Errorf("codeSpans() = %q, want %q", got, one.want)
			}
		})
	}
}

func TestRendererTakesTheSoftWrapSetting(t *testing.T) {
	if renderer(Options{}) == nil || renderer(Options{SoftWraps: true}) == nil {
		t.Fatal("renderer() answered nothing")
	}
}

func TestParse(t *testing.T) {
	if parse([]byte("# T\n\nProse.\n")) == nil {
		t.Error("parse() answered no tree")
	}
}
