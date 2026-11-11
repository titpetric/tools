package mdfmt_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/titpetric/tools/splint/mdfmt"
)

// format runs the formatter and fails the test where it refuses.
func format(t *testing.T, source string) string {
	t.Helper()

	out, err := mdfmt.Format(mdfmt.Options{}, []byte(source))
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	return string(out)
}

func TestFormatSettlesWhitespaceAndMarkers(t *testing.T) {
	for name, one := range map[string]struct{ in, want string }{
		"a paragraph reflows to one line": {
			"# T\n\nOne line here\nand a second line.\n",
			"# T\n\nOne line here and a second line.\n",
		},
		"a gap between words closes to two spaces": {
			"# T\n\nSome     text.\n",
			"# T\n\nSome  text.\n",
		},
		"a list marker is one star and one space": {
			"# T\n\n*   first\n*   second\n",
			"# T\n\n* first\n* second\n",
		},
		"a fenced block keeps its content": {
			"# T\n\n```\n  keep   this\n```\n",
			"# T\n\n```\n  keep   this\n```\n",
		},
		"a blank line between paragraphs stays one": {
			"# T\n\nFirst.\n\n\n\nSecond.\n",
			"# T\n\nFirst.\n\nSecond.\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := format(t, one.in); got != one.want {
				t.Errorf("Format() = %q, want %q", got, one.want)
			}
		})
	}
}

func TestFormatRunsGofmtOverAGoBlock(t *testing.T) {
	got := format(t, "# T\n\n```go\nfunc  x( ) {  }\n```\n")

	if want := "# T\n\n```go\nfunc x() {}\n```\n"; got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}

func TestFormatKeepsFrontMatter(t *testing.T) {
	got := format(t, "---\ntitle: Doc\ndate: \"2026-01-01\"\n---\n# Heading\n\nText.\n")

	want := "---\ntitle: Doc\ndate: \"2026-01-01\"\n---\n\n# Heading\n\nText.\n"
	if got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}

func TestFormatLeavesAnUnclosedFenceAlone(t *testing.T) {
	got := format(t, "---\ntitle: Doc\n\n# Heading\n")

	if got == "" {
		t.Fatal("Format() answered nothing")
	}
	if !slices.Contains([]byte(got), '#') {
		t.Errorf("Format() lost the heading: %q", got)
	}
}

func TestFormatIsSettled(t *testing.T) {
	once := format(t, "# T\n\nSome   text.\n\n*   item\n")

	if twice := format(t, once); twice != once {
		t.Errorf("a second pass moved the file:\n%q\n%q", once, twice)
	}
}

func TestFormatFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.md")

	if err := os.WriteFile(path, []byte("# T\n\nSome   text.\n"), 0o644); err != nil {
		t.Fatalf("writing the file: %v", err)
	}

	moved, err := mdfmt.FormatFile(mdfmt.Options{}, path)
	if err != nil {
		t.Fatalf("FormatFile() error = %v", err)
	}
	if !moved {
		t.Error("FormatFile() reported no change for a file with gaps in it")
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the file: %v", err)
	}
	if got, want := string(body), "# T\n\nSome  text.\n"; got != want {
		t.Errorf("the file holds %q, want %q", got, want)
	}

	moved, err = mdfmt.FormatFile(mdfmt.Options{}, path)
	if err != nil {
		t.Fatalf("FormatFile() error = %v", err)
	}
	if moved {
		t.Error("FormatFile() rewrote a file that was already formatted")
	}
}

func TestFormatFileRefusesWhatIsNotThere(t *testing.T) {
	if _, err := mdfmt.FormatFile(mdfmt.Options{}, filepath.Join(t.TempDir(), "absent.md")); err == nil {
		t.Error("FormatFile() took a path that is not there")
	}
}

// tree writes a fixture of markdown in and under a directory.
func tree(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	for _, path := range []string{
		"top.md", "notes.txt", "a.go",
		"docs/one.md", "docs/deep/two.md",
		"vendor/x/v.md", "node_modules/y/n.md", "testdata/t.md", ".git/g.md",
	} {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("making %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte("# x\n"), 0o644); err != nil {
			t.Fatalf("writing %s: %v", full, err)
		}
	}
	return dir
}

func TestFilesReachesEveryMarkdownAndNothingElse(t *testing.T) {
	dir := tree(t)

	found, err := mdfmt.Files(dir, true)
	if err != nil {
		t.Fatalf("Files() error = %v", err)
	}

	var got []string
	for _, path := range found {
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			t.Fatalf("Rel(): %v", err)
		}
		got = append(got, filepath.ToSlash(rel))
	}
	slices.Sort(got)

	want := []string{"docs/deep/two.md", "docs/one.md", "top.md"}
	if !slices.Equal(got, want) {
		t.Errorf("Files() = %q, want %q", got, want)
	}
}

func TestFilesStopsAtOneDirectory(t *testing.T) {
	dir := tree(t)

	found, err := mdfmt.Files(dir, false)
	if err != nil {
		t.Fatalf("Files() error = %v", err)
	}

	if len(found) != 1 || filepath.Base(found[0]) != "top.md" {
		t.Errorf("Files() = %q, want the one file at the top", found)
	}
}

func TestFilesTakesOneFile(t *testing.T) {
	dir := tree(t)

	for path, want := range map[string]int{"top.md": 1, "notes.txt": 0} {
		found, err := mdfmt.Files(filepath.Join(dir, path), true)
		if err != nil {
			t.Fatalf("Files(%s) error = %v", path, err)
		}
		if len(found) != want {
			t.Errorf("Files(%s) = %q, want %d", path, found, want)
		}
	}
}

func TestFilesRefusesWhatIsNotThere(t *testing.T) {
	if _, err := mdfmt.Files(filepath.Join(t.TempDir(), "absent"), true); err == nil {
		t.Error("Files() took a path that is not there")
	}
}

func TestFormatKeepsSoftWraps(t *testing.T) {
	const wrapped = "# T\n\nOne line here\nand a second line.\n"

	out, err := mdfmt.Format(mdfmt.Options{SoftWraps: true}, []byte(wrapped))
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	if got := string(out); got != wrapped {
		t.Errorf("Format() = %q, want the breaks kept at %q", got, wrapped)
	}

	// The same input without the setting joins the paragraph, which is the
	// difference the setting exists for.
	joined := format(t, wrapped)
	if joined == wrapped {
		t.Errorf("Format() kept the breaks with the setting off: %q", joined)
	}
}

func TestFormatSoftWrapsIsSettled(t *testing.T) {
	opts := mdfmt.Options{SoftWraps: true}

	once, err := mdfmt.Format(opts, []byte("# T\n\nOne line\nand a second.\n\n*   item\n"))
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	twice, err := mdfmt.Format(opts, once)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	if string(twice) != string(once) {
		t.Errorf("a second pass moved the file:\n%q\n%q", once, twice)
	}
}

func TestFormatSoftWrapsSettlesTheRest(t *testing.T) {
	out, err := mdfmt.Format(mdfmt.Options{SoftWraps: true}, []byte("# T\n\n*   item\n\n```go\nfunc  x( ) {  }\n```\n"))
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	want := "# T\n\n* item\n\n```go\nfunc x() {}\n```\n"
	if got := string(out); got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}

func TestFormatLeavesAFrontMatterOnlyFileAlone(t *testing.T) {
	const only = "---\ntitle: Security\nrelated:\n  - /features/\n---\n"

	if got := format(t, only); got != only {
		t.Errorf("Format() = %q, want %q", got, only)
	}
}

// The block passes through as the file holds it. mdox reads it with Hugo's
// page parser and writes YAML back, which reorders the keys and drops the
// quoting; over mobius that moved 77 files this leaves alone.
func TestFormatKeepsFrontMatterVerbatim(t *testing.T) {
	const front = "---\nname: issue-flow\ndescription: \"A thing.\"\nwhen_to_use: \"Another.\"\n---\n"

	got := format(t, front+"\n# H\n\nProse.\n")
	if !strings.HasPrefix(got, front) {
		t.Errorf("Format() rewrote the block:\n%q", got)
	}
}

func TestFormatRefusesToRewriteACodeSpan(t *testing.T) {
	source := []byte("# T\n\nBackticks mark names: `` `Projects::validate` ``, prose does not.\n")

	if _, err := mdfmt.Format(mdfmt.Options{}, source); !errors.Is(err, mdfmt.ErrCodeSpan) {
		t.Errorf("Format() error = %v, want ErrCodeSpan", err)
	}

	// A plain code span round-trips. The guard then stays out of the way.
	if _, err := mdfmt.Format(mdfmt.Options{}, []byte("# T\n\nA `name` here.\n")); err != nil {
		t.Errorf("Format() refused a plain code span: %v", err)
	}
}

func TestFormatFileLeavesACodeSpanFileAlone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.md")
	source := "# T\n\nNames wear backticks: `` `Thing::run` ``.   Gaps stay.\n"

	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("writing the file: %v", err)
	}

	moved, err := mdfmt.FormatFile(mdfmt.Options{}, path)
	if !errors.Is(err, mdfmt.ErrCodeSpan) {
		t.Errorf("FormatFile() error = %v, want ErrCodeSpan", err)
	}
	if moved {
		t.Error("FormatFile() reported a change for a file it refused")
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the file: %v", err)
	}
	if string(body) != source {
		t.Errorf("the file moved:\n%q\n%q", source, body)
	}
}
