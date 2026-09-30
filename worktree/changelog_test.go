package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"strings"
	"testing"
)

// TestChangelogStatus checks the state each tag is reported in: what the remote
// holds against the note, and what that takes.
func TestChangelogStatus(t *testing.T) {
	const note = "# module @ v1.0.0\n\nReleased v1.0.0.\n"

	tests := []struct {
		name    string
		release ghRelease
		found   bool
		pushed  bool
		want    string
	}{
		{"no tag on the remote", ghRelease{}, false, false, changelogUnpushed},
		{"tag without a release", ghRelease{}, false, true, changelogCreate},
		{"release without a body", ghRelease{TagName: "v1.0.0"}, true, true, changelogFill},
		{"release with a blank body", ghRelease{TagName: "v1.0.0", Body: "  \n\n"}, true, true, changelogFill},
		{"release holding the note", ghRelease{TagName: "v1.0.0", Body: note}, true, true, changelogCurrent},
		{"release holding something else", ghRelease{TagName: "v1.0.0", Body: "hand written"}, true, true, changelogDiffers},
		{"draft against an unpushed tag", ghRelease{TagName: "v1.0.0", Body: note, Draft: true}, true, false, changelogCurrent},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := changelogStatus(note, test.release, test.found, test.pushed); got != test.want {
				t.Fatalf("changelogStatus() = %q, want %q", got, test.want)
			}
		})
	}
}

// TestSameBodyIgnoresARoundTrip checks that a note read back from GitHub, which
// rewrites the line endings and trims the end of the body, is not reported as a
// note that has to be written again.
func TestSameBodyIgnoresARoundTrip(t *testing.T) {
	note := "# module @ v1.0.0\n\n| Commit | Subject |\n| --- | --- |\n"
	stored := "# module @ v1.0.0\r\n\r\n| Commit | Subject |   \r\n| --- | --- |"

	if !sameBody(stored, note) {
		t.Fatalf("sameBody() reported a stored note as different:\n%q\n%q", stored, note)
	}
	if sameBody(stored+"\r\nAn extra line.", note) {
		t.Fatal("sameBody() reported a body with an extra line as the same")
	}
}

// TestChangelogBodyLeavesOutVisibility checks the note holds the release and
// not the counts of the working tree, which every note would otherwise repeat.
func TestChangelogBodyLeavesOutVisibility(t *testing.T) {
	v := verdict{
		Module:   "example.com/repo",
		Version:  "v1.0.0",
		Since:    "v0.9.0",
		Released: true,
		Commits:  []commitLog{{Hash: "abc1234", Subject: "do the thing"}},
		Visibility: visibilityReport{
			Packages: []visibilityPackage{{Package: "./", ExportedFuncs: 3}},
		},
	}

	body := changelogBody(v)
	for _, want := range []string{"# example.com/repo @ v1.0.0", "## Commits v0.9.0..v1.0.0", "do the thing"} {
		if !strings.Contains(body, want) {
			t.Errorf("changelogBody() is missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "Visibility") {
		t.Errorf("changelogBody() kept the visibility of the working tree:\n%s", body)
	}
	if !strings.HasSuffix(body, "\n") || strings.HasSuffix(body, "\n\n") {
		t.Errorf("changelogBody() does not end on a single newline: %q", body[max(len(body)-4, 0):])
	}
}

// TestLimitBodyCutsAtALine checks a note too long for GitHub is cut where a
// line ends and says that it was.
func TestLimitBodyCutsAtALine(t *testing.T) {
	line := strings.Repeat("a", 99) + "\n"
	body := strings.Repeat(line, ghBodyLimit/len(line)+10)

	cut := limitBody(body)
	if len(cut) > ghBodyLimit {
		t.Fatalf("limitBody() left %d characters, over the %d GitHub stores", len(cut), ghBodyLimit)
	}
	if !strings.HasSuffix(cut, "characters.\n") {
		t.Fatalf("limitBody() did not say the note was cut: %q", cut[max(len(cut)-80, 0):])
	}

	kept, _, ok := strings.Cut(cut, "\nThe rest of this note was cut")
	if !ok {
		t.Fatalf("limitBody() wrote no notice where it cut: %q", cut[max(len(cut)-80, 0):])
	}
	if len(kept)%len(line) != 0 {
		t.Fatalf("limitBody() kept %d characters, which is not a whole number of %d character lines", len(kept), len(line))
	}
}

// TestLimitBodyLeavesAShortNoteAlone checks the limit costs a note that fits
// nothing at all.
func TestLimitBodyLeavesAShortNoteAlone(t *testing.T) {
	body := "# module @ v1.0.0\n"
	if got := limitBody(body); got != body {
		t.Fatalf("limitBody() = %q, want %q", got, body)
	}
}

// TestChangelogRangesCoverEveryTag checks that each tag gets a note of its own,
// measured from the release below it, and that the working tree gets none.
func TestChangelogRangesCoverEveryTag(t *testing.T) {
	tags := []string{"v0.0.1", "v0.0.2", "v0.1.0", "v0.1.1"}

	got := changelogRanges(tags, "", "")
	want := []versionRange{
		{From: "v0.1.0", To: "v0.1.1"},
		{From: "v0.0.2", To: "v0.1.0"},
		{From: "v0.0.1", To: "v0.0.2"},
		{To: "v0.0.1"},
	}

	if len(got) != len(want) {
		t.Fatalf("changelogRanges() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("changelogRanges()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

// TestChangelogRangesBoundedByATag checks --from and --to narrow the tags a run
// reports on.
func TestChangelogRangesBoundedByATag(t *testing.T) {
	tags := []string{"v0.0.1", "v0.0.2", "v0.1.0", "v0.1.1"}

	got := changelogRanges(tags, "v0.0.2", "v0.1.0")
	want := []versionRange{{From: "v0.0.2", To: "v0.1.0"}}

	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("changelogRanges() = %v, want %v", got, want)
	}
}

// TestChangelogRunRendersThePlan checks a run without --apply writes the reason
// and the command it would run, and nothing else.
func TestChangelogRunRendersThePlan(t *testing.T) {
	run := &changelogRun{}
	entry := changelogEntry{Tag: "v1.0.0", Body: "note", Status: changelogCreate, Latest: true}

	if !run.write(t.TempDir(), entry) {
		t.Fatal("changelogRun.write() reported a failure while rendering the plan")
	}

	got := run.cell()
	for _, want := range []string{
		"No release names the tag.",
		"gh release create v1.0.0 --verify-tag --title v1.0.0 --latest --notes-file -",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("changelogRun.write() cell is missing %q:\n%s", want, got)
		}
	}
	if run.cell() != "" {
		t.Error("changelogRun.cell() did not empty the lines for the next tag")
	}
}

// TestChangelogRunLeavesAFilledReleaseAlone checks that a release already
// holding a body is reported and nothing is proposed for it, whether or not
// that body is the note. A note somebody published is not the run's to rewrite.
func TestChangelogRunLeavesAFilledReleaseAlone(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{changelogCurrent, "The release holds the note already."},
		{changelogDiffers, "The release holds a note of its own, which is left as published."},
		{changelogUnpushed, "The remote does not carry the tag."},
	}

	for _, test := range tests {
		t.Run(test.status, func(t *testing.T) {
			run := &changelogRun{}
			if !run.write(t.TempDir(), changelogEntry{Tag: "v1.0.0", Body: "note", Status: test.status}) {
				t.Fatalf("changelogRun.write() reported a failure for a %s tag", test.status)
			}
			if got := run.cell(); got != test.want {
				t.Fatalf("changelogRun.write() cell = %q, want the reason alone, %q", got, test.want)
			}
		})
	}
}

// TestChangelogWritesOnlyEmptyReleases checks the states --apply acts on are
// the two that hold no note: a tag with no release, and a release published
// empty.
func TestChangelogWritesOnlyEmptyReleases(t *testing.T) {
	for _, status := range changelogStatuses {
		want := status == changelogCreate || status == changelogFill
		if got := changelogWrites[status]; got != want {
			t.Errorf("changelogWrites[%q] = %v, want %v", status, got, want)
		}
	}
}

// TestChangelogRunDoesNotRunGHForAFilledRelease checks that --apply reaches no
// further than the two states it writes, even with gh there to answer.
func TestChangelogRunDoesNotRunGHForAFilledRelease(t *testing.T) {
	out := stubGH(t, "echo ran > \"$STUB_OUT\"\n")

	run := &changelogRun{apply: true}
	if !run.write(t.TempDir(), changelogEntry{Tag: "v1.0.0", Body: "note", Status: changelogDiffers}) {
		t.Fatalf("changelogRun.write() failed: %s", run.cell())
	}

	if _, err := os.Stat(out); err == nil {
		t.Fatal("changelogRun.write() ran gh against a release holding a note of its own")
	}
}

// TestChangelogRunReportsAFailedWrite checks a gh call that fails is painted
// red, keeps its output, and is counted as a failure rather than stopping.
func TestChangelogRunReportsAFailedWrite(t *testing.T) {
	stubGH(t, "echo 'HTTP 404: Not Found'\nexit 1\n")

	run := &changelogRun{apply: true}
	if run.write(t.TempDir(), changelogEntry{Tag: "v1.0.0", Body: "note", Status: changelogFill}) {
		t.Fatal("changelogRun.write() reported a failed gh call as written")
	}

	if got := run.cell(); !strings.Contains(got, "HTTP 404: Not Found") {
		t.Fatalf("changelogRun.write() cell is missing what gh printed:\n%s", got)
	}
}

// TestChangelogRunWritesTheNoteOnStdin checks the body reaches gh on standard
// input rather than as an argument, which is where a note of thousands of
// characters does not fit.
func TestChangelogRunWritesTheNoteOnStdin(t *testing.T) {
	out := stubGH(t, "cat > \"$STUB_OUT\"\n")

	run := &changelogRun{apply: true, verbose: true}
	if !run.write(t.TempDir(), changelogEntry{Tag: "v1.0.0", Body: "the note\n", Status: changelogFill}) {
		t.Fatalf("changelogRun.write() failed: %s", run.cell())
	}

	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != "the note\n" {
		t.Fatalf("gh read %q on standard input, want the note", written)
	}
}

// TestParseOptionsChangelog checks the subcommand is recognised and is not
// mistaken for a path filter.
func TestParseOptionsChangelog(t *testing.T) {
	originalArgs := os.Args
	originalFlags := flag.CommandLine
	t.Cleanup(func() {
		os.Args = originalArgs
		flag.CommandLine = originalFlags
	})

	os.Args = []string{"worktree", commandChangelog, "--apply"}
	flag.CommandLine = flag.NewFlagSet("worktree", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	opts := ParseOptions()
	if !opts.Changelog || !opts.Apply {
		t.Fatalf("ParseOptions(changelog --apply) = %#v, want both set", opts)
	}
	if opts.FilterArg != "" || opts.FilterPath != "" {
		t.Fatalf("ParseOptions() treated %q as a filter: %#v", commandChangelog, opts)
	}
}

// TestChangelogHelpIsDocumented checks the command reaches the help page, which
// is the only place a reader finds it.
func TestChangelogHelpIsDocumented(t *testing.T) {
	fs := flag.NewFlagSet("worktree", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	(&Options{}).bind(fs)

	buf := &bytes.Buffer{}
	if err := writeHelp(buf, helpSpec(fs)); err != nil {
		t.Fatalf("writeHelp() error = %v", err)
	}
	if !strings.Contains(buf.String(), "`"+commandChangelog+"`") {
		t.Fatalf("help page does not list the %s command:\n%s", commandChangelog, buf)
	}
}

// stubGH puts a gh of the test's own on the path, running the script given, and
// returns the file the script writes to through STUB_OUT.
func stubGH(t *testing.T, script string) string {
	t.Helper()

	dir := t.TempDir()
	out := dir + "/stub.out"
	body := "#!/bin/sh\nSTUB_OUT=" + out + "\n" + script
	if err := os.WriteFile(dir+"/gh", []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	return out
}
