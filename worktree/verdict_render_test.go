package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/titpetric/tools/worktree/components"
)

func TestRenderVerdictMarkdown(t *testing.T) {
	var out bytes.Buffer
	renderVerdict(&out, sampleVerdict(), false)

	got := out.String()
	for _, want := range []string{
		"# example.com/x @ v1.1.0",
		"Minor release: v1.1.0, because 1 exported symbol was removed and 1 signature changed and 2 exported fields moved since v1.0.0.",
		"## Commits since v1.0.0",
		"| [`abc1234`](https://github.com/example/x/commit/abc1234) | feat: add Client |",
		"## API since v1.0.0",
		// The package has a column of its own, a module holding one of them
		// included: a symbol with nowhere named is a symbol nobody can find.
		"| Change | Package | Symbol |",
		"| Added | / | type Client struct |",
		"| Changed | / | Before: Open ()<br>After: Open (string) |",
		"| Removed | / | func Legacy () error |",
		// One table, whatever the release did to however many types.
		"## Data model since v1.0.0",
		"| Change | Package | Type | Field |",
		// A type the release adds is written as the shape it declares, and
		// carries the mark that says the type itself is new.
		"| Added | / | type Client struct ▲ | Name string `json:\"name\"` |",
		// A type that was already there is written as what moved on it, and
		// the cells repeating the row above are left empty.
		"|  |  | type Config struct | Timeout int ▲ |",
		"| Changed | / | type Config struct | Addr string `yaml:\"addr\"` -> []string `yaml:\"addr\"` |",
		"| Removed | / | type Config struct | Retries int |",
		// The working tree stands on its own: one row per package, counted
		// and not judged, with the legend naming what the columns hold.
		"## Visibility, the working tree",
		"Counts are the declared types and funcs of each package, split by the case of their name.",
		"| Package | Exported types | Internal types | Exported funcs | Internal funcs | Internal code |",
		"| ./ | 9 | 1 | 43 | 37 | 44.8% |",
		"| ./storage | 3 | 1 | 22 | 3 | 8.4% |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
		}
	}

	if strings.Contains(got, "\033") {
		t.Error("renderVerdict() wrote escape codes into markdown")
	}
	// Both tables carry the column, and the first row of each group fills it.
	for _, table := range []string{"| Change | Package | Symbol |", "| Change | Package | Type | Field |"} {
		rows := tableRows(got, table)
		if len(rows) == 0 {
			t.Fatalf("renderVerdict() wrote no rows under %q:\n%s", table, got)
		}
		if rows[0][1] == "" {
			t.Errorf("renderVerdict() opened %q on an empty package cell: %v\n%s", table, rows[0], got)
		}
	}
}

// tableRows returns the body rows of the markdown table opening on header, each
// split into its cells, so one column can be read on its own.
func tableRows(report, header string) [][]string {
	at := strings.Index(report, header)
	if at < 0 {
		return nil
	}

	var rows [][]string
	for _, line := range strings.Split(report[at:], "\n")[1:] {
		if !strings.HasPrefix(line, "|") {
			break
		}
		if strings.HasPrefix(line, "| --- |") {
			continue
		}
		line = strings.TrimSuffix(strings.TrimPrefix(line, "| "), " |")
		rows = append(rows, strings.Split(line, " | "))
	}
	return rows
}

// TestRenderVerdictMethodReceivers pins the receiver on a changed method: the
// compared signatures carry the bare name, so without the qualification two
// same-named methods on different types read as one change repeated.
func TestRenderVerdictMethodReceivers(t *testing.T) {
	v := sampleVerdict()
	v.API.Changed = []apiChange{{
		Key: "example.com/x.file.Save", Package: "example.com/x",
		Name: "file.Save", Exported: false,
		Old: "Save (context.Context) error", New: "Save (context.Context, string) error",
	}}

	var out bytes.Buffer
	renderVerdict(&out, v, false)

	got := out.String()
	want := "Before: file.Save (context.Context) error<br>After: file.Save (context.Context, string) error"
	if !strings.Contains(got, want) {
		t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
	}
}

func TestRenderVerdictANSI(t *testing.T) {
	var out bytes.Buffer
	renderVerdict(&out, sampleVerdict(), true)

	got := out.String()
	if !strings.Contains(got, "\033") {
		t.Error("renderVerdict() wrote no escape codes to a terminal")
	}
	for _, unwanted := range []string{"<details>", "<summary>", "```", "# example.com/x", "| Change |"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("renderVerdict() wrote markup %q to a terminal:\n%s", unwanted, got)
		}
	}

	plain := ansi.Strip(got)
	for _, want := range []string{
		"example.com/x @ v1.1.0",
		"Commits since v1.0.0",
		"abc1234",
		"Removed",
		"func Legacy () error",
		"Data model since v1.0.0",
		"type Client struct ▲",
		"Name string `json:\"name\"`",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, plain)
		}
	}
	if !strings.Contains(plain, "╭") {
		t.Errorf("renderVerdict() drew no table:\n%s", plain)
	}

	// A heading sits directly on top of the table it names, and the blank line
	// falls after the table, so the two read as one block.
	if !strings.Contains(plain, "Commits since v1.0.0\n╭") {
		t.Errorf("renderVerdict() parted a heading from its table:\n%s", plain)
	}
	if !strings.Contains(plain, "╯\n\nAPI since v1.0.0") {
		t.Errorf("renderVerdict() left no blank line between two tables:\n%s", plain)
	}
	if !strings.HasSuffix(plain, "╯\n\n") {
		t.Errorf("renderVerdict() did not end on a blank line:\n%s", plain)
	}

	// The heading is not written in the colour the columns of the table are, or
	// it would read as another one of them.
	if !strings.Contains(got, components.ColorSection+"API since v1.0.0") {
		t.Errorf("renderVerdict() wrote a heading in another colour:\n%q", got)
	}
}

func TestRenderVerdictWithoutARemoteLeavesHashesUnlinked(t *testing.T) {
	v := sampleVerdict()
	v.RepoURL = ""

	var out bytes.Buffer
	renderVerdict(&out, v, false)

	got := out.String()
	if !strings.Contains(got, "| `abc1234` | feat: add Client |") {
		t.Errorf("renderVerdict() did not fall back to a plain hash:\n%s", got)
	}
	if strings.Contains(got, "](") {
		t.Errorf("renderVerdict() linked a hash without a remote to link into:\n%s", got)
	}
}

func TestRenderVerdictLeavesUnpublishedCommitsUnlinked(t *testing.T) {
	v := sampleVerdict()
	v.Commits[0].Published = false

	var out bytes.Buffer
	renderVerdict(&out, v, false)

	got := out.String()
	if !strings.Contains(got, "| `abc1234` | feat: add Client |") {
		t.Errorf("renderVerdict() did not render the unpublished hash plainly:\n%s", got)
	}
	if strings.Contains(got, v.RepoURL+"/commit/abc1234") {
		t.Errorf("renderVerdict() linked an unpublished commit:\n%s", got)
	}
}

func TestRenderVerdictOmitsEmptySections(t *testing.T) {
	var out bytes.Buffer
	renderVerdict(&out, verdict{Module: "example.com/x", Version: "v1.0.1", Since: "v1.0.0", Release: releasePatch}, false)

	got := out.String()
	if strings.Contains(got, "##") {
		t.Errorf("renderVerdict() wrote a heading with nothing under it:\n%s", got)
	}
	if !strings.Contains(got, "Patch release: v1.0.1") {
		t.Errorf("renderVerdict() did not report the release:\n%s", got)
	}
}

func TestRenderVerdictWritesTheCountColumnsOfEachCommit(t *testing.T) {
	requireSplint(t)

	v, err := readVerdict(commitScanRepo(t), "", "", false)
	if err != nil {
		t.Fatalf("readVerdict() error: %v", err)
	}

	var out bytes.Buffer
	renderVerdict(&out, v, false)
	got := out.String()

	for _, want := range []string{
		"| Commit | External | Internal | Subject |",
		"| +1/~0/-0 |  | alpha: add Greet |",
		// A commit that moved nothing exported leaves the cell empty rather
		// than writing three zeroes.
		"|  | alpha: document the package |",
		"| +0/~1/-0 |  | alpha: greet a number of times |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVerdict() output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderVerdictLeavesTheCommitColumnsOutWhenNothingWasScanned(t *testing.T) {
	var out bytes.Buffer
	renderVerdict(&out, sampleVerdict(), false)
	got := out.String()

	// A range read as one comparison has no commit to attribute a symbol to,
	// and gets no column of empty cells.
	for _, unwanted := range []string{"| Commits |", "| Commit | External | Internal | Subject |"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("renderVerdict() wrote %q for a range that was not scanned:\n%s", unwanted, got)
		}
	}
}
