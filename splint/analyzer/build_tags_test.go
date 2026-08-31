package analyzer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/titpetric/tools/splint/analyzer"
)

func TestBuildTags(t *testing.T) {
	src := `
// +build debug
// +build linux

package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}`

	want := []string{"debug", "linux"}
	got := analyzer.BuildTags([]byte(src))

	assert.Equal(t, want, got)
}
