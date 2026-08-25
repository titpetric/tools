package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/titpetric/tools/worktree/components"
)

// verdict is the release a module has earned, or the one it last had: the
// version it is described at, the commits that went into it, and what became
// of its exported API.
type verdict struct {
	// Module names the module, which is its go module path when it has a
	// go.mod and its directory otherwise.
	Module string

	// Version is the release the report describes, which is the one this
	// module moves to when it is behind its latest tag and the latest tag
	// itself when it is not.
	Version string

	// Released reports that Version is a tag that exists, so the report is
	// of what went into it rather than of what a release would take.
	Released bool

	// Since is the version the comparison is measured from, empty when there
	// was none to measure from.
	Since string

	// Release is releasePatch or releaseMinor, and is empty for a release
	// that already happened.
	Release string

	// Commits are the commits between Since and Version, newest first.
	Commits []commitLog

	// API is the exported symbol difference between the two.
	API apiDiff

	// GoBefore and GoAfter are the go directive at each of the two
	// revisions. Moving to another release series costs a minor.
	GoBefore string
	GoAfter  string

	// RepoURL is the address commits are linked into, empty when the module
	// has no origin to derive one from.
	RepoURL string
}

// readVerdict works out the release the module in dir has earned, or reports
// the one it last had.
//
// A module with commits since its latest tag is measured from that tag to the
// working tree, and the version is bumped as a minor when the release takes
// exported API away and as a patch otherwise. A module level with its tag has
// nothing to propose, so the last release is described instead, measured from
// the release before it. A module with no tag at all is a first release, with
// nothing to compare against.
// The two revisions can be named outright with --from and --to, which is how a
// report is asked for over a range the repository is no longer standing on.
func readVerdict(dir, from, to string) (verdict, error) {
	tags, prefix, err := moduleTags(dir)
	if err != nil {
		return verdict{}, err
	}

	v := verdict{Module: moduleName(dir), RepoURL: repoURL(dir)}

	if from != "" || to != "" {
		return v.between(dir, tags, prefix, from, to)
	}

	latest, found := LatestRelease(tags)
	if !found {
		v.Commits = commitLogSinceTag(dir, "")
		v.API = apiDiff{Skipped: "no release tag to compare against"}
		return v.propose(tags)
	}

	if ahead := commitsSinceTag(dir, prefix+latest.String()); ahead > 0 {
		v.Since = latest.String()
		v.Commits = commitLogSinceTag(dir, prefix+v.Since)
		v.API = compareRefs(dir, prefix+v.Since, "")
		v.GoBefore, v.GoAfter = goVersionAt(dir, prefix+v.Since), goVersionAt(dir, "")
		return v.propose(tags)
	}

	// Level with the tag: the release to report on is the one that was made.
	v.Version = latest.String()
	v.Released = true
	if previous, ok := PreviousRelease(tags); ok {
		v.Since = previous.String()
		v.Commits = commitLogBetween(dir, prefix+v.Since, prefix+v.Version)
		v.API = compareRefs(dir, prefix+v.Since, prefix+v.Version)
		v.GoBefore, v.GoAfter = goVersionAt(dir, prefix+v.Since), goVersionAt(dir, prefix+v.Version)
	} else {
		v.Commits = commitLogSinceTag(dir, "")
		v.API = apiDiff{Skipped: "no earlier release to compare against"}
	}
	return v, nil
}

// between reports on the range the caller named. An empty from falls back to
// the release before the one named by to, and an empty to is the working tree.
//
// A "to" naming a release is reported as that release; anything else is a
// proposal, since there is no tag to call it by.
func (v verdict) between(dir string, tags []string, prefix, from, to string) (verdict, error) {
	fromRef, toRef := taggedRef(from, prefix), taggedRef(to, prefix)

	if from == "" {
		// Measure from whatever came before the revision asked for, which is
		// the release below it when it names one.
		if version, ok := ParseVersion(to); ok {
			if previous, found := PreviousRelease(releasesBelow(tags, version)); found {
				from, fromRef = previous.String(), taggedRef(previous.String(), prefix)
			}
		}
		if from == "" {
			if latest, found := LatestRelease(tags); found {
				from, fromRef = latest.String(), taggedRef(latest.String(), prefix)
			}
		}
	}

	v.Since = from
	v.Commits = commitLogBetween(dir, fromRef, toRef)
	v.API = compareRefs(dir, fromRef, toRef)
	v.GoBefore, v.GoAfter = goVersionAt(dir, fromRef), goVersionAt(dir, toRef)

	if version, ok := ParseVersion(to); ok {
		v.Version = version.String()
		v.Released = true
		return v, nil
	}
	return v.propose(tags)
}

// taggedRef turns a version named on the command line into the tag the
// repository carries for it, and leaves anything else alone so a commit or a
// branch can be named just as well.
func taggedRef(ref, prefix string) string {
	if ref == "" || prefix == "" {
		return ref
	}
	if _, ok := ParseVersion(ref); ok {
		return prefix + ref
	}
	return ref
}

// releasesBelow returns the tags naming a release at or below version, so the
// release before one already made can be found.
func releasesBelow(tags []string, version Version) []string {
	var below []string
	for _, tag := range tags {
		if v, ok := ParseVersion(tag); ok && Compare(v, version) <= 0 {
			below = append(below, tag)
		}
	}
	return below
}

// propose fills in the version a release would move to and what it costs.
func (v verdict) propose(tags []string) (verdict, error) {
	v.Release = releasePatch
	if v.API.Breaking || v.MovedGoSeries() {
		v.Release = releaseMinor
	}
	next, _, err := nextRelease(tags, v.Release)
	if err != nil {
		return verdict{}, err
	}
	v.Version = next.String()
	return v, nil
}

// compareRefs reads the exported API difference between two revisions, asking
// for the sources a type body is printed from.
func compareRefs(dir, oldRef, newRef string) apiDiff {
	if !isGoModule(dir) {
		return apiDiff{Skipped: "not a go module"}
	}
	return apiDiffBetween(dir, oldRef, newRef, true)
}

// moduleName returns the go module path of dir, falling back to the name of
// the directory for anything that is not a go module.
func moduleName(dir string) string {
	if path, err := readModulePath(dir); err == nil {
		return path
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return filepath.Base(abs)
}

// Summary states the verdict in one sentence, giving the reason behind it.
func (v verdict) Summary() string {
	switch {
	case v.Released && v.API.Skipped != "":
		return fmt.Sprintf("Released %s, %s.", v.Version, v.API.Skipped)
	case v.Released && (v.API.Breaking || v.MovedGoSeries()):
		return fmt.Sprintf("Released %s: %s since %s.", v.Version, v.breakage(), v.Since)
	case v.Released:
		return fmt.Sprintf("Released %s: no exported symbols were removed since %s.", v.Version, v.Since)
	case v.Since == "":
		return fmt.Sprintf("First release: %s, with no earlier release to compare against.", v.Version)
	case v.API.Skipped != "" && !v.MovedGoSeries():
		return fmt.Sprintf("Patch release: %s, the API was not compared, %s.", v.Version, v.API.Skipped)
	case v.Release == releaseMinor:
		return fmt.Sprintf("Minor release: %s, because %s since %s.", v.Version, v.breakage(), v.Since)
	default:
		return fmt.Sprintf("Patch release: %s, no exported symbols were removed since %s.", v.Version, v.Since)
	}
}

// MovedGoSeries reports whether the release moves the module to another go
// release series, which stops it building for anyone on the older toolchain. A
// point release of the same series changes nothing for a consumer.
func (v verdict) MovedGoSeries() bool {
	return goSeriesChanged(v.GoBefore, v.GoAfter)
}

// Range names the two revisions the report covers.
func (v verdict) Range() string {
	if v.Since == "" {
		return "the first commit"
	}
	if v.Released {
		return v.Since + ".." + v.Version
	}
	return "since " + v.Since
}

// breakage describes what a release costs, which is what earns it a minor.
func (v verdict) breakage() string {
	var parts []string
	if removed := len(v.API.Removed); removed > 0 {
		parts = append(parts, plural(removed, "exported symbol was removed", "exported symbols were removed"))
	}
	if changed := len(v.API.Changed); changed > 0 {
		parts = append(parts, plural(changed, "signature changed", "signatures changed"))
	}
	if v.MovedGoSeries() {
		parts = append(parts, fmt.Sprintf("go moved from %s to %s", v.GoBefore, v.GoAfter))
	}
	return strings.Join(parts, " and ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// renderVerdict writes the verdict as a terminal report, or as markdown when
// it is not going to a terminal, which is what makes a redirected run paste
// into a release note.
func renderVerdict(w io.Writer, v verdict, styled bool) {
	writeTitle(w, v.Module+" "+v.Version, styled)
	fmt.Fprintf(w, "%s\n", v.Summary())

	wrap := 0
	if styled {
		wrap = terminalWidth(w)
	}

	if len(v.Commits) > 0 {
		writeHeading(w, "Commits "+v.Range(), styled)
		writeCommits(w, v, styled, wrap)
	}
	if headers, rows := symbolRows(v, styled, wrap); len(rows) > 0 {
		writeHeading(w, "API "+v.Range(), styled)
		writeSimpleTable(w, headers, rows, styled)
	}
	writeDefinitions(w, v.API, styled)
}

// writeTitle writes the line the report opens on.
func writeTitle(w io.Writer, title string, styled bool) {
	if !styled {
		fmt.Fprintf(w, "# %s\n\n", title)
		return
	}
	fmt.Fprintf(w, "%s\n", colorLines(title, components.ColorWhite, styled))
}

// writeHeading writes a section heading, as markdown when the output is not a
// terminal and as a colored line when it is. Either way it opens on a blank
// line, so the sections stay apart.
func writeHeading(w io.Writer, title string, styled bool) {
	if !styled {
		fmt.Fprintf(w, "\n## %s\n\n", title)
		return
	}
	fmt.Fprintf(w, "\n%s\n", colorLines(title, components.ColorHeader, styled))
}

// writeCommits writes the commit table. The hash links into the repository in
// markdown, and stands on its own in a terminal, where a URL is only noise.
func writeCommits(w io.Writer, v verdict, styled bool, wrap int) {
	subject := cellWidth(wrap, []int{shortHashWidth(v.Commits), 0})

	rows := make([][]string, 0, len(v.Commits))
	for _, commit := range v.Commits {
		hash := "`" + commit.Hash + "`"
		if styled {
			hash = colorLines(commit.Hash, components.ColorTeal, true)
		} else if v.RepoURL != "" {
			hash = fmt.Sprintf("[`%s`](%s/commit/%s)", commit.Hash, v.RepoURL, commit.Hash)
		}
		rows = append(rows, []string{hash, fold(commit.Subject, subject)})
	}
	writeSimpleTable(w, []string{"Commit", "Subject"}, rows, styled)
}

// shortHashWidth returns the width of the hash column.
func shortHashWidth(commits []commitLog) int {
	width := len("Commit")
	for _, commit := range commits {
		width = max(width, len(commit.Hash))
	}
	return width
}

// symbolEntry is one symbol as a row of the API table.
type symbolEntry struct {
	category string
	pkg      string
	text     string
}

// symbolRows returns one row per symbol, in the order a reader wants the
// categories: what the release takes away first, what it adds last.
//
// The category names only the first row of its group and the rows after it
// leave the column empty. The table draws no rule between rows, so a group
// reads as one block under its heading.
//
// A module holding more than one package gains a column naming which one a
// symbol belongs to, without which "const Name" three times over says nothing.
func symbolRows(v verdict, styled bool, wrap int) ([]string, [][]string) {
	var entries []symbolEntry
	for _, symbol := range v.API.Removed {
		entries = append(entries, symbolEntry{"Removed", symbol.Package, symbol.String()})
	}
	for _, change := range v.API.Changed {
		entries = append(entries, symbolEntry{"Changed", change.Package, change.Old + " -> " + change.New})
	}
	for _, symbol := range v.API.Added {
		entries = append(entries, symbolEntry{"Added", symbol.Package, symbol.String()})
	}
	if len(entries) == 0 {
		return nil, nil
	}

	colors := map[string]string{
		"Removed": components.ColorRed,
		"Changed": components.ColorAmber,
		"Added":   components.ColorGreen,
	}

	headers := []string{"Change", "Symbol"}
	widths := []int{len("Removed")}
	packages := packageColumn(v, entries)
	if packages {
		headers = []string{"Change", "Package", "Symbol"}
		width := len("Package")
		for _, entry := range entries {
			width = max(width, len(entry.pkg))
		}
		widths = append(widths, width)
	}
	symbolWidth := cellWidth(wrap, append(widths, 0))

	var (
		rows     [][]string
		previous string
	)
	for _, entry := range entries {
		category := ""
		if entry.category != previous {
			category = colorLines(entry.category, colors[entry.category], styled)
			previous = entry.category
		}

		row := []string{category}
		if packages {
			row = append(row, colorLines(entry.pkg, components.ColorSeparator, styled))
		}
		rows = append(rows, append(row, fold(entry.text, symbolWidth)))
	}
	return headers, rows
}

// packageColumn shortens every entry to its path below the module and reports
// whether the entries span more than one package, which is when naming them is
// worth a column.
func packageColumn(v verdict, entries []symbolEntry) bool {
	seen := make(map[string]bool)
	for i, entry := range entries {
		pkg := strings.TrimPrefix(strings.TrimPrefix(entry.pkg, v.Module), "/")
		entries[i].pkg = pkg
		seen[pkg] = true
	}
	return len(seen) > 1
}

// fold breaks a line to a width, and leaves it alone when there is no width to
// break it to.
func fold(line string, width int) string {
	if width <= 0 {
		return line
	}
	return ansi.Wrap(line, width, "")
}

// writeDefinitions writes the body of every type the release adds or changes
// the shape of, so a reader sees what was declared rather than only its name.
//
// A grouped "type (...)" block is one declaration under several names, so the
// same body would otherwise be written once per name.
func writeDefinitions(w io.Writer, diff apiDiff, styled bool) {
	seen := make(map[string]bool)
	for _, symbol := range diff.Added {
		if symbol.Definition == "" || seen[symbol.Definition] {
			continue
		}
		seen[symbol.Definition] = true

		if !styled {
			fmt.Fprintf(w, "\n<details>\n<summary><code>%s %s</code></summary>\n\n", symbol.Kind, symbol.Name)
			fmt.Fprintf(w, "```go\n%s\n```\n\n</details>\n", symbol.Definition)
			continue
		}

		fmt.Fprintf(w, "\n%s\n", colorLines(symbol.Kind+" "+symbol.Name, components.ColorHeader, styled))
		for _, line := range strings.Split(symbol.Definition, "\n") {
			fmt.Fprintf(w, "  %s\n", colorLines(line, components.ColorSeparator, styled))
		}
	}
}
