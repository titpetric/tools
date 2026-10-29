package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/titpetric/tools/worktree/components"
)

// What the release note of one tag takes; a changelog run reports one row per
// tag with this status.
//
// The run writes to empty releases only. Readers of a release read the body
// somebody published, and a note generated from the history is no reason to
// take it away, so the run reports the two states holding one and leaves them
// alone.
const (
	// changelogCreate is a tag the remote carries no release for, which --apply
	// creates.
	changelogCreate = "create"

	// changelogFill is a release that is there with nothing written in it,
	// which --apply writes the note into.
	changelogFill = "fill"

	// changelogCurrent is a release whose body is the note already.
	changelogCurrent = "current"

	// changelogDiffers is a release holding a body other than the note, which
	// the run leaves as published.
	changelogDiffers = "differs"

	// changelogUnpushed is a tag that exists here and not on the remote, so no
	// release can exist for it.
	changelogUnpushed = "unpushed"
)

// changelogStatuses are the states a row reports; the status column takes its
// width from them.
var changelogStatuses = []string{changelogCreate, changelogFill, changelogCurrent, changelogDiffers, changelogUnpushed}

// changelogWrites are the states --apply acts on, which is a release that holds
// no note of its own.
var changelogWrites = map[string]bool{changelogCreate: true, changelogFill: true}

// ghBodyLimit is the longest release body GitHub stores, in characters. The run
// cuts a longer note and still publishes it, since a truncated note still reads.
const ghBodyLimit = 125000

// changelogEntry is the release note of one tag: the note the report writes for
// it, and what the remote holds against it.
type changelogEntry struct {
	// Tag is the tag as the repository carries it, the "<subdir>/" prefix of a
	// nested module included, the name the release goes by.
	Tag string

	// Body is the note, which is the markdown worktree verdict writes for the
	// release.
	Body string

	// Status is what the note takes: nothing, a release, or a body.
	Status string

	// Latest reports that the tag is the newest release of the repository,
	// which is the one release that carries the "Latest" badge.
	Latest bool
}

// changelog reports the release note of every tag of the repository in dir, one
// row per tag, and under --apply writes the notes of the releases that hold
// none: the tags with no release at all, and the releases published empty.
//
// changelog never writes to a release that already holds a body. Readers of
// that release read that body, and a note generated from the history is no
// reason to take it away.
//
// changelog reads every release through one model cache, the way the release
// chain reads one, so a tag that ends one range and starts the next gets
// unpacked and modelled once.
//
// --from and --to bound the tags reported on, naming the releases the run works
// between, the same way they bound a release chain.
func changelog(w io.Writer, dir string, opts *Options, styled bool) error {
	tags, prefix, err := moduleTags(dir)
	if err != nil {
		return err
	}

	ranges := changelogRanges(tags, opts.From, opts.To)
	if len(ranges) == 0 {
		return fmt.Errorf("no release falls in the range asked for")
	}

	releases, err := ghReleases(dir)
	if err != nil {
		return err
	}
	pushed, err := ghTags(dir)
	if err != nil {
		return err
	}

	models, err := newAPIModels(!opts.NoCache)
	if err != nil {
		return err
	}
	defer func() { _ = models.Close() }()

	latest, _ := LatestRelease(tags)
	base := verdict{Module: moduleName(dir), RepoURL: repoURL(dir)}

	headers := []string{"Tag", "Status", "Note"}
	widths := []int{
		columnWidth("Tag", changelogTags(ranges, prefix)),
		columnWidth("Status", changelogStatuses),
		0,
	}

	run := &changelogRun{apply: opts.Apply, verbose: opts.Verbose, styled: styled}
	if styled {
		run.wrap = cellWidth(terminalWidth(w), widths)
	}

	table := newStreamTable(w, headers, widths, styled)
	defer table.close()

	failed := 0
	for _, r := range ranges {
		v, err := base.report(dir, tags, prefix, r.From, r.To, true, models)
		if err != nil {
			return err
		}

		tag := prefix + r.To
		release, found := releases[tag]
		entry := changelogEntry{
			Tag:    tag,
			Body:   changelogBody(v),
			Latest: r.To == latest.String(),
		}
		entry.Status = changelogStatus(entry.Body, release, found, pushed[tag])

		table.start(
			colorLines(tag, components.ColorTeal, styled),
			colorLines(entry.Status, changelogColor(entry.Status), styled),
		)
		if !run.write(dir, entry) {
			failed++
		}
		table.finish(run.cell())
	}

	if failed > 0 {
		return fmt.Errorf("%s could not be written", plural(failed, "release note", "release notes"))
	}
	return nil
}

// changelogRanges returns the ranges the notes come from, one per release,
// newest first. Every tag gets a note of its own, measured from the release
// below it: a note belongs to the tag it publishes under, so the series the
// release chain collapses patches into is no use here.
//
// changelogRanges drops the working tree, since the working tree is no release
// and has no tag to publish a note under.
func changelogRanges(tags []string, from, upTo string) []versionRange {
	var ranges []versionRange
	for _, r := range releaseChain(tags, false, true, from, upTo) {
		if r.To != "" {
			ranges = append(ranges, r)
		}
	}
	return ranges
}

// changelogTags names the tag of every range; the tag column takes its width
// from them.
func changelogTags(ranges []versionRange, prefix string) []string {
	tags := make([]string, 0, len(ranges))
	for _, r := range ranges {
		tags = append(tags, prefix+r.To)
	}
	return tags
}

// changelogBody renders the release note of one verdict, which is the markdown
// worktree verdict writes with the visibility table left out: that table counts
// the working tree and ignores the release, so it would say the same thing in
// every note, and in all but the newest note its counts would not match the
// release.
func changelogBody(v verdict) string {
	v.Visibility = visibilityReport{}

	out := &strings.Builder{}
	renderVerdict(out, v, false)
	return limitBody(strings.TrimSpace(out.String()) + "\n")
}

// limitBody cuts a note too long for GitHub to store at a line boundary, and
// says so at the cut. limitBody counts bytes against a limit in characters,
// which errs on the short side and never on the rejected one.
func limitBody(body string) string {
	if len(body) <= ghBodyLimit {
		return body
	}

	notice := "\nThe rest of this note was cut: GitHub stores a release body of at most " +
		strconv.Itoa(ghBodyLimit) + " characters.\n"
	cut := max(ghBodyLimit-len(notice), 0)
	body = body[:cut]
	if end := strings.LastIndex(body, "\n"); end > 0 {
		body = body[:end+1]
	}
	return body + notice
}

// changelogStatus reports what a tag's note takes, given the release the remote
// carries under the tag and whether the tag is on the remote at all.
//
// The run fills an existing release for any tag, since GitHub
// publishes a draft against a tag that does not have to exist yet. The run
// reports a tag with no release and no place on the remote and leaves it
// alone.
//
// changelogStatus compares a release holding a body of its own against the
// note and leaves the release alone either way. The comparison earns the run
// no work and shows up in the report for the reader: a published note out of
// step with the history tells the reader to look, and what to do about it
// stays the reader's decision.
func changelogStatus(body string, release ghRelease, found, pushed bool) string {
	switch {
	case !found && !pushed:
		return changelogUnpushed
	case !found:
		return changelogCreate
	case strings.TrimSpace(release.Body) == "":
		return changelogFill
	case sameBody(release.Body, body):
		return changelogCurrent
	default:
		return changelogDiffers
	}
}

// sameBody reports whether a release body holds the note already.
//
// GitHub stores a body with CRLF line endings and trims what follows the last
// line of it, so sameBody compares the normalized lines of both and ignores
// byte differences. Without this every note reads as out of date the moment it
// goes out.
func sameBody(a, b string) bool {
	return normalizeBody(a) == normalizeBody(b)
}

// normalizeBody rewrites a body as the lines it holds, with the line endings
// and the trailing whitespace a round trip through GitHub changes taken out.
func normalizeBody(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")

	lines := strings.Split(body, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// changelogColor paints the status column: amber for a tag with a note to
// write, red for one with no possible note, green for a release already
// holding the note, and grey for one holding a note of its own, which the run
// reads and does not act on.
func changelogColor(status string) string {
	switch {
	case changelogWrites[status]:
		return components.ColorAmber
	case status == changelogUnpushed:
		return components.ColorRed
	case status == changelogCurrent:
		return components.ColorGreen
	default:
		return components.ColorSeparator
	}
}

// changelogReason says why a row reads the way it does, which the one word of
// the status column has no room for.
func changelogReason(status string) string {
	switch status {
	case changelogUnpushed:
		return "The remote does not carry the tag."
	case changelogCreate:
		return "No release names the tag."
	case changelogFill:
		return "The release holds no note."
	case changelogDiffers:
		return "The release holds a note of its own, which is left as published."
	default:
		return "The release holds the note already."
	}
}

// changelogRun collects the lines of one row's note column, and writes the note
// to the remote under --apply.
//
// The gh command runs with its working directory set to the repository it
// belongs to, so no step ever changes the directory worktree itself is in.
type changelogRun struct {
	apply   bool
	verbose bool
	styled  bool

	// wrap is the width the cell folds at, and is zero when the output is
	// not going to a terminal and folding it would only get in the way.
	wrap int

	// lines are the cell of the tag in the current row.
	lines []string
}

// write records what a row takes and performs it under --apply. It reports
// whether the row came out right, which a tag with nothing to write does.
//
// A failed note does not stop the run: the releases are independent, and the
// tag after a rejected one is no less publishable for it. The run counts the
// failures, and the count becomes the exit status once every tag has been through.
func (r *changelogRun) write(dir string, entry changelogEntry) (ok bool) {
	r.add(components.ColorSeparator, "%s", changelogReason(entry.Status))
	if !changelogWrites[entry.Status] {
		return true
	}

	note := releaseNote{
		Tag:    entry.Tag,
		Title:  entry.Tag,
		Body:   entry.Body,
		Create: entry.Status == changelogCreate,
		Latest: entry.Latest,
	}

	line := shellJoin(note.args())
	if !r.apply {
		r.add("", "%s", line)
		return true
	}

	out, err := writeReleaseNote(dir, note)
	if err != nil {
		r.add(components.ColorRed, "%s", line)
		r.output(out)
		return false
	}

	r.add("", "%s%s", line, r.check())
	if r.verbose {
		r.output(out)
	}
	return true
}

// add appends a colored line, folded to the width left for the cell.
func (r *changelogRun) add(color, format string, args ...any) {
	text := fmt.Sprintf(format, args...)
	if r.wrap > 0 {
		text = ansi.Wrap(text, r.wrap, "")
	}
	r.lines = append(r.lines, colorLines(text, color, r.styled))
}

// cell returns the lines collected for a tag, and empties them for the next.
func (r *changelogRun) cell() string {
	cell := strings.Join(r.lines, "\n")
	r.lines = nil
	return cell
}

// check is the mark a command that ran carries, the one resolve uses for the
// same purpose.
func (r *changelogRun) check() string {
	if !r.styled {
		return ""
	}
	return " " + components.ColorGreen + "✓" + components.ColorReset
}

// output writes what gh printed, indented under the command.
func (r *changelogRun) output(out string) {
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			r.add(components.ColorSeparator, "  %s", line)
		}
	}
}
