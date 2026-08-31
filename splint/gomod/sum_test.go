package gomod_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/titpetric/tools/splint/gomod"
	"github.com/titpetric/tools/splint/model"
)

// sums is the go.sum the tests read: one version hashed twice, one recorded by
// its go.mod alone, and a second module.
const sums = `example.com/a v1.0.0 h1:one=
example.com/a v1.0.0/go.mod h1:two=
example.com/a v1.1.0/go.mod h1:three=
example.com/b v2.0.0 h1:four=
example.com/b v2.0.0/go.mod h1:five=
`

// writeSum writes a go.sum beside a go.mod and returns the directory.
func writeSum(t *testing.T, contents string) string {
	t.Helper()

	dir := writeModule(t, "module example.com/x\n\ngo 1.24.0\n")
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), []byte(contents), 0o644); err != nil {
		t.Fatalf("writing go.sum: %v", err)
	}
	return dir
}

// TestReadSum covers the two lines a version can have. The hash of the source
// and the hash of the requirements are one version, and which of them is
// present says whether the build downloads it.
func TestReadSum(t *testing.T) {
	dir := writeSum(t, sums)

	got, err := gomod.ReadSum(filepath.Join(dir, "go.sum"))
	if err != nil {
		t.Fatalf("gomod.ReadSum() error: %v", err)
	}

	want := []model.Sum{
		{Path: "example.com/a", Version: "v1.0.0", Zip: true},
		{Path: "example.com/a", Version: "v1.1.0"},
		{Path: "example.com/b", Version: "v2.0.0", Zip: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("gomod.ReadSum() = %#v, want %#v", got, want)
	}
}

// TestReadCarriesSums covers reading the go.sum beside the go.mod, which is
// where the versions reach the model from.
func TestReadCarriesSums(t *testing.T) {
	dir := writeSum(t, sums)

	got, err := gomod.Read(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatalf("gomod.Read() error: %v", err)
	}
	if len(got.Sums) != 3 {
		t.Errorf("Sums = %#v, want the three versions of the go.sum", got.Sums)
	}
}

// TestReadWithoutSums covers a module that has never been built. It has no
// go.sum, and that is not an error.
func TestReadWithoutSums(t *testing.T) {
	dir := writeModule(t, "module example.com/x\n\ngo 1.24.0\n")

	got, err := gomod.Read(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatalf("gomod.Read() error: %v", err)
	}
	if got.Sums != nil {
		t.Errorf("Sums = %#v, want none", got.Sums)
	}
}
