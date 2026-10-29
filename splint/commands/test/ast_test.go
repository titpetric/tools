package test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTestKindOf covers how a function in a test file is read: by the name
// and by the parameter the testing package gives it, so a helper named like a
// test is a helper.
func TestTestKindOf(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kinds_test.go")
	source := `package p

import "testing"

func TestOne(t *testing.T) {}
func BenchmarkOne(b *testing.B) {}
func FuzzOne(f *testing.F) {}
func Example() {}
func TestHelper(a int) {}
func helper() {}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	parsed, err := parseFile(path, "kinds_test.go", ".")
	if err != nil {
		t.Fatalf("parseFile() error = %v", err)
	}

	want := map[string]testKind{
		"TestOne":      kindTest,
		"BenchmarkOne": kindBenchmark,
		"FuzzOne":      kindFuzz,
		"Example":      kindExample,
		"TestHelper":   kindFunc,
		"helper":       kindFunc,
	}
	for _, fn := range parsed.Funcs {
		if got, found := want[fn.Name]; found && fn.Kind != got {
			t.Errorf("%s read as %v, want %v", fn.Name, fn.Kind, got)
		}
	}
}

// TestFuncKey covers a symbol's key: the receiver qualified name for a method
// and the bare name for a function.
func TestFuncKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.go")
	source := `package p

type Cache struct{}

func (c *Cache) Set(key string) {}
func (c Cache) Get(key string) {}
func Open() *Cache { return nil }
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	parsed, err := parseFile(path, "keys.go", ".")
	if err != nil {
		t.Fatalf("parseFile() error = %v", err)
	}

	want := map[string]string{
		"Set":  "Cache.Set",
		"Get":  "Cache.Get",
		"Open": "Open",
	}
	for _, fn := range parsed.Funcs {
		if key := want[fn.Name]; key != "" && fn.Key != key {
			t.Errorf("%s has key %q, want %q", fn.Name, fn.Key, key)
		}
	}
}
