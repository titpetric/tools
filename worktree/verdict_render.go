package main

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/titpetric/tools/worktree/components"
)

// renderVerdict writes the verdict as a terminal report, or as markdown when
// it is not going to a terminal, which is what makes a redirected run paste
// into a release note.
func renderVerdict(w io.Writer, v verdict, styled bool) {
	writeTitle(w, v.Module+" @ "+v.Version, styled)
	fmt.Fprintf(w, "%s\n", v.Summary())
	if v.Scope == scopeRoot {
		fmt.Fprintf(w, "The API is read from the root package alone.\n")
	}
	writeGap(w, styled)

	wrap := 0
	if styled {
		wrap = terminalWidth(w)
	}

	if len(v.Commits) > 0 {
		writeHeading(w, "Commits "+v.Range(), styled)
		writeCommits(w, v, styled, wrap)
		writeGap(w, styled)
	}
	if headers, rows := symbolRows(v, styled, wrap); len(rows) > 0 {
		writeHeading(w, "API "+v.Range(), styled)
		writeSimpleTable(w, headers, rows, styled)
		writeGap(w, styled)
	}
	writeDataModel(w, v, styled, wrap)
	writeDependencies(w, v, styled)
	writeVisibility(w, v, styled)
}

// writeVisibility writes what each package declares and how much of it is
// private, one row per package.
//
// The counts are reported and not judged. There is no share of internal code a
// package ought to carry: a parser is mostly private and a data model mostly
// not, and both are as they should be. What the table is for is reading one
// package against another, and against what the same package was a release
// ago.
func writeVisibility(w io.Writer, v verdict, styled bool) {
	if len(v.Visibility.Packages) == 0 {
		return
	}

	headers := []string{"Package", "Exported types", "Internal types", "Exported funcs", "Internal funcs", "Internal code"}
	rows := make([][]string, 0, len(v.Visibility.Packages))
	for _, pkg := range v.Visibility.Packages {
		rows = append(rows, []string{
			colorLines(pkg.Package, components.ColorSeparator, styled),
			strconv.Itoa(pkg.ExportedTypes),
			strconv.Itoa(pkg.InternalTypes),
			strconv.Itoa(pkg.ExportedFuncs),
			strconv.Itoa(pkg.InternalFuncs),
			fmt.Sprintf("%.1f%%", pkg.InternalRatio),
		})
	}

	writeHeading(w, "Visibility, the working tree", styled)
	writeLegend(w, "Counts are the declared types and funcs of each package, split by the case of their name. Internal code is the share of the package's code inside internal func bodies.", styled)
	writeSimpleTable(w, headers, rows, styled)
	writeGap(w, styled)
}

// writeLegend writes the sentence naming what a table's columns hold, between
// the heading and the table.
func writeLegend(w io.Writer, text string, styled bool) {
	if !styled {
		fmt.Fprintf(w, "%s\n\n", text)
		return
	}
	fmt.Fprintf(w, "%s\n", colorLines(text, components.ColorSeparator, styled))
}

// writeTitle writes the line the report opens on, which names the module and
// the release it is reporting on, and stands clear of the summary under it.
func writeTitle(w io.Writer, title string, styled bool) {
	if !styled {
		fmt.Fprintf(w, "# %s\n\n", title)
		return
	}
	fmt.Fprintf(w, "%s\n\n", colorLines(title, components.ColorTitle, styled))
}

// writeHeading writes a section heading, as markdown when the output is not a
// terminal and as a colored line when it is.
//
// A terminal heading sits directly on top of the table it names, which is what
// makes the two read as one block: the blank line separating it from the block
// above is written by that block, as it ends.
func writeHeading(w io.Writer, title string, styled bool) {
	if !styled {
		fmt.Fprintf(w, "\n## %s\n\n", title)
		return
	}
	fmt.Fprintf(w, "%s\n", colorLines(title, components.ColorSection, styled))
}

// writeGap ends a block of a terminal report with a blank line, so a heading
// and its table stay together and the sections stay apart. Markdown takes its
// spacing from the heading opening the next section instead.
func writeGap(w io.Writer, styled bool) {
	if styled {
		fmt.Fprintln(w)
	}
}

// writeCommits writes the commit table. The hash links into the repository in
// markdown, and stands on its own in a terminal, where a URL is only noise.
//
// A commit that moved symbols says so in the counts the stats table reads the
// whole release in, so the one commit behind a removal can be picked out of a
// run of twenty. External counts what a consumer can see and Internal what it
// cannot, which is what tells a release from a refactor. A commit that moved
// neither leaves the cells empty rather than writing three zeroes, since a
// table of "+0/~0/-0" hides the rows that matter.
func writeCommits(w io.Writer, v verdict, styled bool, wrap int) {
	external, internal := commitCounts(v, styled)

	headers := []string{"Commit", "Subject"}
	widths := []int{shortHashWidth(v.Commits), 0}
	counted := external != nil || internal != nil
	if counted {
		headers = []string{"Commit", "External", "Internal", "Subject"}
		widths = []int{
			shortHashWidth(v.Commits),
			columnWidth("External", slices.Collect(maps.Values(external))),
			columnWidth("Internal", slices.Collect(maps.Values(internal))),
			0,
		}
	}
	subject := cellWidth(wrap, widths)

	rows := make([][]string, 0, len(v.Commits))
	for _, commit := range v.Commits {
		row := []string{commitLink(v, commit.Hash, styled)}
		if counted {
			row = append(row, external[commit.Hash], internal[commit.Hash])
		}
		rows = append(rows, append(row, fold(commit.Subject, subject)))
	}
	writeSimpleTable(w, headers, rows, styled)
}

// commitCounts renders the two count columns of the commit table, keyed on the
// short hash: what a commit did to the API, and what it did to everything a
// consumer cannot reach. Either map is nil when no commit of the range moved
// that half, since a column of empty cells adds nothing.
func commitCounts(v verdict, styled bool) (external, internal map[string]string) {
	external = make(map[string]string, len(v.Commits))
	internal = make(map[string]string, len(v.Commits))

	for _, commit := range v.Commits {
		diff, ok := v.CommitAPI[commit.Hash]
		if !ok || diff.Skipped != "" {
			continue
		}
		if counts := symbolCounts(diff, true, styled); counts != "" {
			external[commit.Hash] = counts
		}
		if counts := symbolCounts(diff, false, styled); counts != "" {
			internal[commit.Hash] = counts
		}
	}
	if len(external) == 0 {
		external = nil
	}
	if len(internal) == 0 {
		internal = nil
	}
	return external, internal
}

// symbolCounts renders one half of what a commit moved as the triple the stats
// table counts a release in: what it added, what it reshaped, what it took
// away. A half that moved nothing renders as nothing.
func symbolCounts(diff apiDiff, exported bool, styled bool) string {
	var added, changed, removed int
	for _, symbol := range diff.Added {
		if symbol.Exported == exported {
			added++
		}
	}
	for _, change := range diff.Changed {
		if change.Exported == exported {
			changed++
		}
	}
	for _, symbol := range diff.Removed {
		if symbol.Exported == exported {
			removed++
		}
	}
	if added+changed+removed == 0 {
		return ""
	}
	return colorLines("+"+strconv.Itoa(added), components.ColorGreen, styled) +
		"/" + colorLines("~"+strconv.Itoa(changed), components.ColorAmber, styled) +
		"/" + colorLines("-"+strconv.Itoa(removed), components.ColorRed, styled)
}

// commitLink renders a commit hash the way the tables name it: a link into the
// repository in markdown, and a coloured hash on a terminal.
func commitLink(v verdict, hash string, styled bool) string {
	if styled {
		return colorLines(hash, components.ColorTeal, true)
	}
	if v.RepoURL == "" || !commitPublished(v.Commits, hash) {
		return "`" + hash + "`"
	}
	return fmt.Sprintf("[`%s`](%s/commit/%s)", hash, v.RepoURL, hash)
}

func commitPublished(commits []commitLog, hash string) bool {
	for _, commit := range commits {
		if commit.Hash == hash {
			return commit.Published
		}
	}
	return false
}

// commitLinks names every commit behind one row, in the order they were made.
func commitLinks(v verdict, hashes []string, styled bool) string {
	links := make([]string, 0, len(hashes))
	for _, hash := range hashes {
		links = append(links, commitLink(v, hash, styled))
	}
	return strings.Join(links, ", ")
}

// commitCells renders the commits behind each row of a table, and returns nil
// when there are none to name: a range nothing was attributed in gets no
// column rather than an empty one.
func commitCells(v verdict, rows [][]string, styled bool) []string {
	cells := make([]string, len(rows))
	named := false
	for i, hashes := range rows {
		cells[i] = commitLinks(v, hashes, styled)
		named = named || cells[i] != ""
	}
	if !named {
		return nil
	}
	return cells
}

// shortHashWidth returns the width of the hash column.
func shortHashWidth(commits []commitLog) int {
	width := len("Commit")
	for _, commit := range commits {
		width = max(width, len(commit.Hash))
	}
	return width
}

// columnWidth returns the width a column of rendered cells takes, which is the
// widest of them and the header naming it. The colours a terminal cell carries
// are not part of what it takes up.
func columnWidth(header string, cells []string) int {
	width := ansi.StringWidth(header)
	for _, cell := range cells {
		width = max(width, ansi.StringWidth(cell))
	}
	return width
}
