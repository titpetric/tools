package coverprofile_test

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/cover"

	"github.com/titpetric/tools/splint/coverprofile"
	"github.com/titpetric/tools/splint/model"
)

// document is one package of three functions, on the lines the profiles below
// name: Half runs one statement of two, None runs neither of its two, and Empty
// has nothing to run.
func document() *model.DocumentRoot {
	decl := func(name string, line, lines, cognitive int) *model.Declaration {
		return &model.Declaration{
			Kind:       model.FuncKind,
			Name:       name,
			File:       "thing.go",
			Line:       line,
			Complexity: &model.Complexity{Cognitive: cognitive, Cyclomatic: 1, Lines: lines},
		}
	}

	return &model.DocumentRoot{
		SchemaVersion: model.SchemaVersion,
		Packages: model.DefinitionList{
			{
				Package: model.Package{
					Package:    "thing",
					ImportPath: "example.com/thing",
					Path:       ".",
				},
				Funcs: model.DeclarationList{
					decl("Half", 10, 6, 2),
					decl("None", 20, 5, 3),
					decl("Empty", 30, 2, 0),
				},
			},
		},
	}
}

// profiles are the blocks of the same file: one of two statements that ran and
// one that did not inside Half, and two that did not run inside None. Nothing
// falls in the lines Empty occupies.
func profiles() []*cover.Profile {
	return []*cover.Profile{
		{
			FileName: "example.com/thing/thing.go",
			Mode:     "set",
			Blocks: []cover.ProfileBlock{
				{StartLine: 11, EndLine: 12, NumStmt: 1, Count: 4},
				{StartLine: 13, EndLine: 14, NumStmt: 1, Count: 0},
				{StartLine: 21, EndLine: 22, NumStmt: 2, Count: 0},
			},
		},
	}
}

func find(t *testing.T, root *model.DocumentRoot, name string) *model.Declaration {
	t.Helper()

	decl := root.Packages[0].Funcs.Find(func(d *model.Declaration) bool { return d.Name == name })
	if decl == nil {
		t.Fatalf("the document holds no %s", name)
	}
	return decl
}

// TestOverlay covers the coverage of every declaration: the blocks that fall
// inside its lines, weighted by the statements in them.
func TestOverlay(t *testing.T) {
	root := document()
	coverprofile.Overlay(root, profiles())

	for name, want := range map[string]float64{
		"Half":  50,
		"None":  0,
		"Empty": 0,
	} {
		if got := find(t, root, name).Complexity.Coverage; got != want {
			t.Errorf("%s coverage = %v, want %v", name, got, want)
		}
	}
}

// TestOverlayWeightsByStatement covers what "statement weighted" means: a
// package of one covered statement in four is 25%, where the mean of the three
// functions is 16.67%.
func TestOverlayWeightsByStatement(t *testing.T) {
	root := document()
	coverprofile.Overlay(root, profiles())

	pkg := root.Packages[0].Package
	if pkg.Complexity == nil {
		t.Fatal("the package carries no complexity")
	}
	if got := pkg.Complexity.Coverage; got != 25 {
		t.Errorf("package coverage = %v, want 25", got)
	}
}

// TestOverlaySumsComplexity covers the rest of the package complexity: nothing
// else fills it, and a report printing coverage beside complexity reads both
// off the same struct.
func TestOverlaySumsComplexity(t *testing.T) {
	root := document()
	coverprofile.Overlay(root, profiles())

	pkg := root.Packages[0].Package
	if pkg.Complexity.Cognitive != 5 {
		t.Errorf("package cognitive = %d, want 5", pkg.Complexity.Cognitive)
	}
	if pkg.Complexity.Cyclomatic != 3 {
		t.Errorf("package cyclomatic = %d, want 3", pkg.Complexity.Cyclomatic)
	}
	if pkg.Complexity.Lines != 13 {
		t.Errorf("package lines = %d, want 13", pkg.Complexity.Lines)
	}
}

// TestOverlayOfAnotherPackage covers a profile naming a file of a package the
// document does not hold: the blocks are keyed by import path, so they reach
// nothing here.
func TestOverlayOfAnotherPackage(t *testing.T) {
	root := document()
	coverprofile.Overlay(root, []*cover.Profile{{
		FileName: "example.com/other/thing.go",
		Blocks:   []cover.ProfileBlock{{StartLine: 11, EndLine: 12, NumStmt: 1, Count: 1}},
	}})

	if got := root.Packages[0].Package.Complexity.Coverage; got != 0 {
		t.Errorf("package coverage = %v, want 0", got)
	}
}

// TestOverlayOfNothing covers the calls a caller does not have to guard.
func TestOverlayOfNothing(t *testing.T) {
	coverprofile.Overlay(nil, profiles())
	coverprofile.Overlay(document(), nil)
}

// TestApply covers reading the profile off disk, in the format "go test
// -coverprofile" writes.
func TestApply(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pkg.cov")
	written := "mode: set\n" +
		"example.com/thing/thing.go:11.2,12.3 1 4\n" +
		"example.com/thing/thing.go:13.2,14.3 1 0\n"
	if err := os.WriteFile(path, []byte(written), 0o644); err != nil {
		t.Fatal(err)
	}

	root := document()
	if err := coverprofile.Apply(root, path); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := find(t, root, "Half").Complexity.Coverage; got != 50 {
		t.Errorf("Half coverage = %v, want 50", got)
	}
}

// TestApplyMissingFile covers the profile that is not there, which is named in
// the error so a pipeline says which step did not write it.
func TestApplyMissingFile(t *testing.T) {
	err := coverprofile.Apply(document(), filepath.Join(t.TempDir(), "nowhere.cov"))
	if err == nil {
		t.Fatal("Apply() accepted a profile that does not exist")
	}
}
