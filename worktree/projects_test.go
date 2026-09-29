package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/titpetric/tools/worktree/config"
)

func TestFindScanRootUsesNearestMarker(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.work"), "go 1.25\n")
	module := filepath.Join(root, "apps", "booking")
	writeTestFile(t, filepath.Join(module, "go.mod"), "module example.com/booking\n\ngo 1.25\n")
	start := filepath.Join(module, "cmd", "server")
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := findScanRoot(start, config.Default().Scan.RootMarkers)
	if err != nil {
		t.Fatal(err)
	}
	if got != module {
		t.Fatalf("findScanRoot() = %q, want %q", got, module)
	}
}

func TestFindProjectsIncludesGitRepositoriesAndGoModules(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/root\n\ngo 1.25\n")

	if err := os.MkdirAll(filepath.Join(root, "standalone", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "standalone", "nested", "go.mod"), "module example.com/nested\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, "module-only", "go.mod"), "module example.com/module-only\n\ngo 1.25\n")

	got, err := findProjects(root, config.Default().Scan)
	if err != nil {
		t.Fatal(err)
	}
	want := []projectDir{
		{Path: ".", GoModule: true, GitRepo: true},
		{Path: "." + string(filepath.Separator) + "module-only", GoModule: true},
		{Path: "." + string(filepath.Separator) + "standalone", GitRepo: true},
		{Path: "." + string(filepath.Separator) + filepath.Join("standalone", "nested"), GoModule: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findProjects() = %#v, want %#v", got, want)
	}
}

func TestFindProjectsSkipsIgnoredDirectories(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/root\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, ".gitignore"), "vendor/\n/tmp\n!apps/vendor\n")

	// Git repositories and Go modules inside an ignored folder are skipped.
	if err := os.MkdirAll(filepath.Join(root, "vendor", "example.com", "lib", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "vendor", "example.com", "lib", "go.mod"), "module example.com/lib\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, "tmp", "scratch", "go.mod"), "module example.com/scratch\n\ngo 1.25\n")

	// A negation in the same file re-includes the folder.
	writeTestFile(t, filepath.Join(root, "apps", "vendor", "go.mod"), "module example.com/apps/vendor\n\ngo 1.25\n")

	// An unanchored pattern matches at any depth, an anchored one does not.
	writeTestFile(t, filepath.Join(root, "libs", "vendor", "go.mod"), "module example.com/libs/vendor\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, "libs", "tmp", "go.mod"), "module example.com/libs/tmp\n\ngo 1.25\n")

	got, err := findProjects(root, config.Default().Scan)
	if err != nil {
		t.Fatal(err)
	}
	want := []projectDir{
		{Path: ".", GoModule: true},
		{Path: "." + string(filepath.Separator) + filepath.Join("apps", "vendor"), GoModule: true},
		{Path: "." + string(filepath.Separator) + filepath.Join("libs", "tmp"), GoModule: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findProjects() = %#v, want %#v", got, want)
	}
}

// TestFindProjectsWithoutGitignore checks the setting that turns the
// .gitignore rules off. A workspace that consolidates git checkouts below it
// gitignores those folders so they stay out of its own index, and with the
// rules on their checkouts are never listed.
func TestFindProjectsWithoutGitignore(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".gitignore"), "checkouts/\n")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "checkouts", "lib", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "checkouts", "lib", "go.mod"), "module example.com/lib\n\ngo 1.25\n")

	scan := config.Default().Scan
	nested := projectDir{
		Path:     "." + string(filepath.Separator) + filepath.Join("checkouts", "lib"),
		GoModule: true,
		GitRepo:  true,
	}

	// With the rules on, the checkout is hidden.
	got, err := findProjects(root, scan)
	if err != nil {
		t.Fatal(err)
	}
	if want := []projectDir{{Path: ".", GitRepo: true}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findProjects() = %#v, want %#v", got, want)
	}

	// With them off, it is listed.
	scan.EnableGitignore = false
	got, err = findProjects(root, scan)
	if err != nil {
		t.Fatal(err)
	}
	want := []projectDir{{Path: ".", GitRepo: true}, nested}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findProjects() = %#v, want %#v", got, want)
	}
}

// TestFindProjectsSkipsIgnorePaths checks the configured ignore paths exclude
// a directory no .gitignore mentions, which is how a listing is kept clean
// once the .gitignore rules are off.
func TestFindProjectsSkipsIgnorePaths(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/root\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, "node_modules", "dep", "go.mod"), "module example.com/dep\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, "apps", "go.mod"), "module example.com/apps\n\ngo 1.25\n")

	scan := config.Default().Scan
	scan.EnableGitignore = false
	scan.IgnorePaths = []string{"node_modules"}

	got, err := findProjects(root, scan)
	if err != nil {
		t.Fatal(err)
	}
	want := []projectDir{
		{Path: ".", GoModule: true},
		{Path: "." + string(filepath.Separator) + "apps", GoModule: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findProjects() = %#v, want %#v", got, want)
	}
}

// TestFindProjectsSkipsTestdataAndGitContents checks the two directories the
// walk never descends into. A go.mod under testdata is a fixture, and one
// under .git belongs to git, so neither is a workspace module that -u may
// rewrite.
func TestFindProjectsSkipsTestdataAndGitContents(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/root\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, "testdata", "go.mod"), "module example.com/fixture\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, "testdata", "nested", "go.mod"), "module example.com/fixture/nested\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, ".git", "modules", "dep", "go.mod"), "module example.com/dep\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, "apps", "testdata", "go.mod"), "module example.com/apps/fixture\n\ngo 1.25\n")
	writeTestFile(t, filepath.Join(root, "apps", "go.mod"), "module example.com/apps\n\ngo 1.25\n")

	got, err := findProjects(root, config.Default().Scan)
	if err != nil {
		t.Fatal(err)
	}
	want := []projectDir{
		{Path: ".", GoModule: true, GitRepo: true},
		{Path: "." + string(filepath.Separator) + "apps", GoModule: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findProjects() = %#v, want %#v", got, want)
	}
}

// TestFindProjectsWithoutGitRepos checks the setting that drops git
// repositories holding no go module, leaving a go only listing.
func TestFindProjectsWithoutGitRepos(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/root\n\ngo 1.25\n")
	if err := os.MkdirAll(filepath.Join(root, "docs", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	scan := config.Default().Scan
	scan.EnableGitRepos = false

	got, err := findProjects(root, scan)
	if err != nil {
		t.Fatal(err)
	}
	if want := []projectDir{{Path: ".", GoModule: true}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findProjects() = %#v, want %#v", got, want)
	}
}

// TestFindScanRootUsesConfiguredMarkers checks the root markers come from the
// configuration, and that a walk with none starts where it was asked to.
func TestFindScanRootUsesConfiguredMarkers(t *testing.T) {
	root := t.TempDir()
	start := filepath.Join(root, "nested", "deep")
	writeTestFile(t, filepath.Join(root, "go.work"), "go 1.25\n")
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := findScanRoot(start, []string{"go.work"})
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("findScanRoot() = %q, want %q", got, root)
	}

	// A marker the workspace does not hold walks up to the filesystem root
	// and falls back to the directory it started in.
	if got, err = findScanRoot(start, []string{"Cargo.toml"}); err != nil || got != start {
		t.Fatalf("findScanRoot() = %q, %v, want %q, nil", got, err, start)
	}
	if got, err = findScanRoot(start, nil); err != nil || got != start {
		t.Fatalf("findScanRoot() with no markers = %q, %v, want %q, nil", got, err, start)
	}
}

func TestFindProjectsIncludesGoWorkUseOutsideRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "workspace")
	external := filepath.Join(parent, "external")
	writeTestFile(t, filepath.Join(root, "go.work"), "go 1.25\n\nuse ../external\n")
	writeTestFile(t, filepath.Join(external, "go.mod"), "module example.com/external\n\ngo 1.25\n")

	got, err := findProjects(root, config.Default().Scan)
	if err != nil {
		t.Fatal(err)
	}
	want := []projectDir{{Path: "." + string(filepath.Separator) + filepath.Join("..", "external"), GoModule: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findProjects() = %#v, want %#v", got, want)
	}
}
