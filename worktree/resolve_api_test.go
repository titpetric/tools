package main

import (
	"archive/tar"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// requireGoFsck skips a test that cannot run without the extraction tool.
func requireGoFsck(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("go-fsck"); err != nil {
		t.Skip("go-fsck is not installed")
	}
}

func TestRepoPaths(t *testing.T) {
	root := testRepo(t, "alpha")

	gotRoot, rel, err := repoPaths(filepath.Join(root, "alpha"))
	if err != nil {
		t.Fatalf("repoPaths() error: %v", err)
	}
	// The temporary directory may be reached through a symlink, so the roots
	// are compared after resolving one.
	wantRoot, _ := filepath.EvalSymlinks(root)
	if got, _ := filepath.EvalSymlinks(gotRoot); got != wantRoot {
		t.Errorf("repoPaths() root = %q, want %q", gotRoot, wantRoot)
	}
	if rel != "alpha" {
		t.Errorf("repoPaths() rel = %q, want %q", rel, "alpha")
	}

	if _, rel, err = repoPaths(root); err != nil || rel != "." {
		t.Errorf("repoPaths() at the root = %q, %v, want \".\", nil", rel, err)
	}

	if _, _, err := repoPaths(t.TempDir()); err == nil {
		t.Error("repoPaths() accepted a directory outside a git repository")
	}
}

func TestGitArchiveUnpacksTheModuleSubtreeOnly(t *testing.T) {
	root := testRepo(t, "alpha", "beta")
	runGit(t, root, "tag", "alpha/v0.1.0")

	// The tag has to be read as it was, not as the working tree is now.
	writeTestFile(t, filepath.Join(root, "alpha", "alpha.go"), "package alpha\n\n// later\n")

	dest := filepath.Join(t.TempDir(), "out")
	if err := gitArchive(filepath.Join(root, "alpha"), "alpha/v0.1.0", dest); err != nil {
		t.Fatalf("gitArchive() error: %v", err)
	}

	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if want := []string{"alpha.go", "go.mod"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("gitArchive() wrote %#v, want %#v", names, want)
	}

	content, err := os.ReadFile(filepath.Join(dest, "alpha.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "later") {
		t.Errorf("gitArchive() wrote the working tree rather than the tag:\n%s", content)
	}
}

func TestUntarRefusesEntriesOutsideTheDestination(t *testing.T) {
	var buf bytes.Buffer
	w := tar.NewWriter(&buf)
	if err := w.WriteHeader(&tar.Header{Name: "../escaped.go", Typeflag: tar.TypeReg, Size: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := untar(&buf, dest); err == nil {
		t.Fatal("untar() accepted an entry outside the destination")
	}
}

func TestFirstLine(t *testing.T) {
	tests := map[string]string{
		"":                    "no output",
		"\n\n":                "no output",
		"  first  \nsecond":   "first",
		"\n\nsecond\nthird\n": "second",
	}
	for input, want := range tests {
		if got := firstLine(input); got != want {
			t.Errorf("firstLine(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestApiDiffSinceTagSkipsWhatItCannotCompare(t *testing.T) {
	requireGoFsck(t)

	if got := apiDiffSinceTag(t.TempDir(), "v9.9.9"); got.Skipped == "" || got.Breaking {
		t.Fatalf("apiDiffSinceTag() against a tag that does not exist = %#v, want a skipped, non breaking result", got)
	}
}

// A comparison measured from the start of history has no revision to read on
// the old side, and reports everything the module exports as added.
func TestApiDiffFromTheStartOfHistory(t *testing.T) {
	requireGoFsck(t)

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")
	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Greet greets.\nfunc Greet(name string) string { return name }\n\n// Bye parts.\nfunc Bye(name string) string { return name }\n")

	got := apiDiffBetween(alpha, "", "")
	if got.Skipped != "" {
		t.Fatalf("apiDiffBetween() skipped: %s", got.Skipped)
	}
	if got.Breaking || len(got.Removed) != 0 {
		t.Errorf("apiDiffBetween() = %#v, want nothing removed: there was nothing to remove", got)
	}

	var names []string
	for _, symbol := range got.Added {
		names = append(names, symbol.Name)
	}
	if want := []string{"Bye", "Greet"}; !reflect.DeepEqual(names, want) {
		t.Errorf("apiDiffBetween() Added = %v, want %v", names, want)
	}
}

func TestApiDiffSinceTagReadsRemovedAndAddedSymbols(t *testing.T) {
	requireGoFsck(t)

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")
	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Greet greets.\nfunc Greet(name string) string { return name }\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: add Greet")
	runGit(t, root, "tag", "alpha/v0.1.0")

	// Adding on its own is not breaking.
	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Greet greets.\nfunc Greet(name string) string { return name }\n\n// Bye parts.\nfunc Bye(name string) string { return name }\n")
	got := apiDiffSinceTag(alpha, "alpha/v0.1.0")
	if got.Skipped != "" {
		t.Fatalf("apiDiffSinceTag() skipped: %s", got.Skipped)
	}
	if got.Breaking {
		t.Errorf("apiDiffSinceTag() called an added symbol breaking: %#v", got)
	}
	want := []apiSymbol{{
		Key:       "example.com/alpha.Bye",
		Package:   "example.com/alpha",
		Name:      "Bye",
		Kind:      "func",
		Signature: "func Bye (name string) string",
	}}
	if !reflect.DeepEqual(got.Added, want) {
		t.Errorf("apiDiffSinceTag() Added = %#v, want %#v", got.Added, want)
	}

	// Taking one away is.
	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Bye parts.\nfunc Bye(name string) string { return name }\n")
	got = apiDiffSinceTag(alpha, "alpha/v0.1.0")
	if got.Skipped != "" {
		t.Fatalf("apiDiffSinceTag() skipped: %s", got.Skipped)
	}
	if !got.Breaking {
		t.Errorf("apiDiffSinceTag() did not call a removed symbol breaking: %#v", got)
	}
	want = []apiSymbol{{
		Key:       "example.com/alpha.Greet",
		Package:   "example.com/alpha",
		Name:      "Greet",
		Kind:      "func",
		Signature: "func Greet (name string) string",
	}}
	if !reflect.DeepEqual(got.Removed, want) {
		t.Errorf("apiDiffSinceTag() Removed = %#v, want %#v", got.Removed, want)
	}
}

func TestApiModelsReadsEachRevisionOnce(t *testing.T) {
	requireGoFsck(t)

	root := testRepo(t, "alpha")
	alpha := filepath.Join(root, "alpha")
	writeTestFile(t, filepath.Join(alpha, "alpha.go"), "package alpha\n\n// Greet greets.\nfunc Greet(name string) string { return name }\n")
	runGit(t, root, "commit", "--quiet", "-am", "alpha: add Greet")
	runGit(t, root, "tag", "alpha/v0.1.0")

	models, err := newAPIModels(false)
	if err != nil {
		t.Fatalf("newAPIModels(false) error: %v", err)
	}
	t.Cleanup(func() { _ = models.Close() })

	first, err := models.model(alpha, "alpha/v0.1.0")
	if err != nil {
		t.Fatalf("model() error: %v", err)
	}
	second, err := models.model(alpha, "alpha/v0.1.0")
	if err != nil {
		t.Fatalf("model() error: %v", err)
	}
	if first != second {
		t.Errorf("model() read the same revision twice: %q then %q", first, second)
	}

	// The working tree is the one revision that can change under a run, so it
	// is read where it stands every time.
	tree, err := models.model(alpha, "")
	if err != nil {
		t.Fatalf("model() error: %v", err)
	}
	again, err := models.model(alpha, "")
	if err != nil {
		t.Fatalf("model() error: %v", err)
	}
	if tree == again {
		t.Error("model() cached the working tree, which can change under a run")
	}

	// A revision the repository does not carry fails, and stays failed.
	if _, err := models.model(alpha, "alpha/v9.9.9"); err == nil {
		t.Error("model() accepted a tag the repository does not carry")
	}
	if _, err := models.model(alpha, "alpha/v9.9.9"); err == nil {
		t.Error("model() accepted a tag it had already refused")
	}

	if err := models.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	if _, err := os.Stat(models.work); !os.IsNotExist(err) {
		t.Errorf("Close() left %s behind", models.work)
	}
}

func TestApiDiffSummaryAndSymbols(t *testing.T) {
	diff := apiDiff{
		Removed: []apiSymbol{{Key: "example.com/x.Gone", Name: "Gone", Kind: "type"}},
		Added:   []apiSymbol{{Key: "example.com/x.New", Name: "New", Kind: "func", Signature: "func New () *Client"}},
		Changed: []apiChange{{Key: "example.com/x.Moved", Old: "Moved ()", New: "Moved (int)"}},
	}
	if want := "api: 1 added, 1 changed, 1 removed"; diff.Summary() != want {
		t.Errorf("Summary() = %q, want %q", diff.Summary(), want)
	}
	want := []string{
		"+ example.com/x.New",
		"~ example.com/x.Moved",
		"    Moved ()",
		"    Moved (int)",
		"- example.com/x.Gone",
	}
	if got := diff.Symbols(); !reflect.DeepEqual(got, want) {
		t.Errorf("Symbols() = %#v, want %#v", got, want)
	}

	// The fields a release moves are counted alongside the symbols, and are
	// left out of the line when it moved none.
	diff.Types = []apiTypeChange{{
		Name: "Config", Underlying: "struct",
		Fields: []apiFieldChange{
			{Name: "Addr", Change: fieldChanged},
			{Name: "Retries", Change: fieldRemoved},
			{Name: "Timeout", Change: fieldAdded},
			{Name: "Tries", Change: fieldAdded},
		},
	}}
	if want := "api: 1 added, 1 changed, 1 removed; fields: 2 added, 1 changed, 1 removed"; diff.Summary() != want {
		t.Errorf("Summary() = %q, want %q", diff.Summary(), want)
	}
	if added, changed, removed := diff.FieldCounts(); added != 2 || changed != 1 || removed != 1 {
		t.Errorf("FieldCounts() = %d, %d, %d, want 2, 1, 1", added, changed, removed)
	}

	skipped := apiDiff{Skipped: "not a go module"}
	if want := "api: not compared, not a go module"; skipped.Summary() != want {
		t.Errorf("Summary() = %q, want %q", skipped.Summary(), want)
	}
}
