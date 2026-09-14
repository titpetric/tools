package sizes_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/commands/sizes"
	"github.com/titpetric/tools/splint/model"
)

// TestData pins the report to the go-ddd-stats contract: the file names carry
// the directory, the root package reads ".", the directories group the files,
// and the histogram buckets run from "< 1 KB" to "> 256 KB".
func TestData(t *testing.T) {
	defs := model.DefinitionList{
		{
			Package: model.Package{Path: "."},
			Files: model.FileList{
				{Name: "main.go", Size: 512},
				{Name: "help.go", Size: 2048},
			},
		},
		{
			Package: model.Package{Path: ".", TestPackage: true},
			Files: model.FileList{
				{Name: "main_test.go", Size: 300 << 10},
			},
		},
		{
			Package: model.Package{Path: "./frontend"},
			Files: model.FileList{
				{Name: "view.go", Size: 4096},
			},
		},
	}

	stats := sizes.Data(defs)

	if len(stats.Files) != 4 {
		t.Fatalf("Data() = %d files, want 4", len(stats.Files))
	}
	if got := stats.Files[0]; got.Name != "frontend/view.go" || got.Path != "frontend" || got.Package != "frontend" {
		t.Errorf("Data() first file = %+v, want frontend/view.go under frontend", got)
	}
	if got := stats.Files[2]; got.Name != "main.go" || got.Path != "" || got.Package != "." || got.Size != 512 {
		t.Errorf("Data() root file = %+v, want main.go at the root as package %q", got, ".")
	}

	if len(stats.Packages) != 2 {
		t.Fatalf("Data() = %d packages, want 2", len(stats.Packages))
	}
	var root *sizes.Package
	for _, pkg := range stats.Packages {
		if pkg.Path == "" {
			root = pkg
		}
	}
	if root == nil || root.Count != 3 || root.Size != 512+2048+(300<<10) {
		t.Errorf("Data() root package = %+v, want the three root files added up", root)
	}

	if len(stats.Histogram) != 10 {
		t.Fatalf("Data() = %d buckets, want 10", len(stats.Histogram))
	}
	counts := map[string]int64{}
	for _, bucket := range stats.Histogram {
		counts[bucket.Size] = bucket.Count
	}
	want := map[string]int64{"< 1 KB": 1, "< 4 KB": 1, "< 8 KB": 1, "> 256 KB": 1}
	for size, count := range want {
		if counts[size] != count {
			t.Errorf("Data() histogram %q = %d, want %d", size, counts[size], count)
		}
	}

	encoded, err := json.Marshal(stats)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"Files"`, `"Packages"`, `"Histogram"`, `"Name"`, `"Path"`, `"Package"`, `"Size"`, `"Count"`, `"Average"`} {
		if !strings.Contains(string(encoded), key) {
			t.Errorf("Marshal() carries no %s key", key)
		}
	}
}
