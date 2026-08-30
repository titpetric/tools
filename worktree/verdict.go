package main

import (
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
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

	// CommitAPI is what each of the commits did to the exported API on its
	// own, keyed on its short hash. It is nil for a range that was not read
	// commit by commit, which is one the tool could not scan.
	CommitAPI map[string]apiDiff

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
// the release before it. A module with no tag at all is a first release,
// measured from the start of history, where everything it exports is reported
// as added.
// The two revisions can be named outright with --from and --to, which is how a
// report is asked for over a range the repository is no longer standing on.
//
// The cached flag is whether the models of the commits it reads are kept
// between runs, which --no-cache turns off.
func readVerdict(dir, from, to string, cached bool) (verdict, error) {
	tags, prefix, err := moduleTags(dir)
	if err != nil {
		return verdict{}, err
	}

	v := verdict{Module: moduleName(dir), RepoURL: repoURL(dir)}

	// One cache holds the range and every commit inside it, so a commit that
	// ends one step and starts the next is modelled once.
	models, err := newAPIModels(cached)
	if err != nil {
		return verdict{}, err
	}
	defer func() { _ = models.Close() }()

	if from != "" || to != "" {
		return v.between(dir, tags, prefix, from, to, models)
	}

	latest, found := LatestRelease(tags)
	if !found {
		return v.report(dir, tags, prefix, "", "", models)
	}

	if ahead := commitsSinceTag(dir, prefix+latest.String()); ahead > 0 {
		return v.report(dir, tags, prefix, latest.String(), "", models)
	}

	// Level with the tag: the release to report on is the one that was made,
	// measured from the release before it, or from the start of history when it
	// is the first one. Both are empty here, so from is the one or the other.
	if previous, ok := PreviousRelease(tags); ok {
		from = previous.String()
	}
	return v.report(dir, tags, prefix, from, latest.String(), models)
}

// between reports on the range the caller named. An empty from falls back to
// the release before the one named by to, and an empty to is the working tree.
// The range it settles on is reported by report.
func (v verdict) between(dir string, tags []string, prefix, from, to string, models *apiModels) (verdict, error) {
	if from == "" {
		// Measure from whatever came before the revision asked for, which is
		// the release below it when it names one.
		if version, ok := ParseVersion(to); ok {
			if previous, found := PreviousRelease(releasesBelow(tags, version)); found {
				from = previous.String()
			}
		}
		if from == "" {
			if latest, found := LatestRelease(tags); found {
				from = latest.String()
			}
		}
	}

	return v.report(dir, tags, prefix, from, to, models)
}

// report fills in the verdict for a range whose two ends are already settled,
// where an empty from is the start of history and an empty to is the working
// tree. Nothing is filled in for either end: a caller naming both, as the
// release chain does, gets the range it asked for.
//
// A range starting at the start of history has no earlier revision to measure
// against, so everything the module exports at the far end of it is reported as
// added, which is what a first release adds.
//
// A "to" naming a release is reported as that release; anything else is a
// proposal, since there is no tag to call it by.
//
// The models are the cache the comparison reads revisions through, and may be
// nil, in which case the revisions are read for this range alone.
func (v verdict) report(dir string, tags []string, prefix, from, to string, models *apiModels) (verdict, error) {
	fromRef, toRef := taggedRef(from, prefix), taggedRef(to, prefix)

	v.Since = from
	v.Commits = commitLogBetween(dir, fromRef, toRef)
	v.API = compareRefs(dir, fromRef, toRef, models)
	v.CommitAPI = scanCommits(dir, fromRef, v.Commits, models)
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

// compareRefs reads the exported API difference between two revisions. The
// models are the cache the revisions are read through, and may be nil for a
// comparison standing alone.
func compareRefs(dir, oldRef, newRef string, models *apiModels) apiDiff {
	if !isGoModule(dir) {
		return apiDiff{Skipped: "not a go module"}
	}
	if models == nil {
		return apiDiffBetween(dir, oldRef, newRef)
	}
	return models.diff(dir, oldRef, newRef)
}

// scanCommits reads what each commit of a range did to the exported API on its
// own, by comparing it against the commit under it.
//
// The commits are walked oldest first, each measured from the one before it,
// so the scans of a range add up to the difference across it: what one commit
// adds and the next takes away is an addition and a removal here, and neither
// there. The commit under the oldest one is the revision the range is measured
// from, which for the start of history is a module holding no packages, the
// same base the range itself is read against.
//
// The commits are those that touched the module, so a commit elsewhere in the
// repository is neither listed nor read. It cannot have moved the API: the
// model is extracted from the module subtree alone.
func scanCommits(dir, fromRef string, commits []commitLog, models *apiModels) map[string]apiDiff {
	if models == nil || len(commits) == 0 || !isGoModule(dir) {
		return nil
	}

	scans := make(map[string]apiDiff, len(commits))
	base := fromRef
	for i := len(commits) - 1; i >= 0; i-- {
		hash := commits[i].Hash
		scans[hash] = models.diff(dir, base, hash)
		base = hash
	}
	return scans
}

// eachCommit calls fn for every commit of the range that was scanned, oldest
// first, which is the order the commits behind a symbol are named in.
func (v verdict) eachCommit(fn func(hash string, diff apiDiff)) {
	for i := len(v.Commits) - 1; i >= 0; i-- {
		hash := v.Commits[i].Hash
		if diff, ok := v.CommitAPI[hash]; ok && diff.Skipped == "" {
			fn(hash, diff)
		}
	}
}

// commitsBySymbol returns the commits that touched each exported symbol,
// oldest first, keyed on the key the comparison reports the symbol under. A
// commit touches a symbol when it introduces it, reshapes it or takes it away.
func (v verdict) commitsBySymbol() map[string][]string {
	touched := make(map[string][]string)
	v.eachCommit(func(hash string, diff apiDiff) {
		for _, symbol := range diff.Added {
			touched[symbol.Key] = appendCommit(touched[symbol.Key], hash)
		}
		for _, change := range diff.Changed {
			touched[change.Key] = appendCommit(touched[change.Key], hash)
		}
		for _, symbol := range diff.Removed {
			touched[symbol.Key] = appendCommit(touched[symbol.Key], hash)
		}
	})
	return touched
}

// commitsByField returns the commits that touched each exported field, oldest
// first, keyed on the type it belongs to and its name.
//
// A commit that adds a type carries every field the type declares with it, the
// same way the data model table reads the fields of a new type as additions of
// their own.
func (v verdict) commitsByField() map[string][]string {
	touched := make(map[string][]string)
	v.eachCommit(func(hash string, diff apiDiff) {
		for _, symbol := range addedTypes(diff) {
			for _, field := range symbol.Fields {
				key := fieldKey(symbol.Key, field.Name)
				touched[key] = appendCommit(touched[key], hash)
			}
		}
		for _, change := range diff.Types {
			for _, field := range change.Fields {
				key := fieldKey(change.Key, field.Name)
				touched[key] = appendCommit(touched[key], hash)
			}
		}
	})
	return touched
}

// fieldKey names one exported field of one type, which is what the data model
// table lists a row per.
func fieldKey(typeKey, field string) string {
	return typeKey + "." + field
}

// appendCommit adds a commit to the ones behind a symbol, unless it is the one
// already there: a symbol that a single commit both reshapes and moves is that
// commit's work once.
func appendCommit(commits []string, hash string) []string {
	if len(commits) > 0 && commits[len(commits)-1] == hash {
		return commits
	}
	return append(commits, hash)
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
	case v.Released && v.Since == "":
		return fmt.Sprintf("Released %s: the first release, %s.", v.Version, v.firstRelease())
	case v.Since == "":
		return fmt.Sprintf("First release: %s, %s.", v.Version, v.firstRelease())
	case v.Released && v.API.Skipped != "":
		return fmt.Sprintf("Released %s, %s.", v.Version, v.API.Skipped)
	case v.Released && (v.API.Breaking || v.MovedGoSeries()):
		return fmt.Sprintf("Released %s: %s since %s.", v.Version, v.breakage(), v.Since)
	case v.Released:
		return fmt.Sprintf("Released %s: no exported symbols were removed since %s.", v.Version, v.Since)
	case v.API.Skipped != "" && !v.MovedGoSeries():
		return fmt.Sprintf("Patch release: %s, the API was not compared, %s.", v.Version, v.API.Skipped)
	case v.Release == releaseMinor:
		return fmt.Sprintf("Minor release: %s, because %s since %s.", v.Version, v.breakage(), v.Since)
	default:
		return fmt.Sprintf("Patch release: %s, no exported symbols were removed since %s.", v.Version, v.Since)
	}
}

// firstRelease describes a release with nothing before it, which is measured
// against a module holding no packages at all: everything it exports is an
// addition, and there is nothing it can have taken away.
func (v verdict) firstRelease() string {
	if v.API.Skipped != "" {
		return "the API was not read, " + v.API.Skipped
	}
	return plural(len(v.API.Added), "exported symbol is added", "exported symbols are added")
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
		if v.Released {
			return "up to " + v.Version
		}
		return "since the first commit"
	}
	if v.Released {
		return v.Since + ".." + v.Version
	}
	return "since " + v.Since
}

// breakage describes what a release costs, which is what earns it a minor.
//
// The data model is counted alongside the symbols: a release that only takes an
// exported field away costs a consumer as much as one that takes a func away,
// and would otherwise be a minor with no reason given for it.
func (v verdict) breakage() string {
	var parts []string
	if removed := len(v.API.Removed); removed > 0 {
		parts = append(parts, plural(removed, "exported symbol was removed", "exported symbols were removed"))
	}
	if changed := len(v.API.Changed); changed > 0 {
		parts = append(parts, plural(changed, "signature changed", "signatures changed"))
	}
	if fields := v.API.BreakingFields(); fields > 0 {
		parts = append(parts, plural(fields, "exported field moved", "exported fields moved"))
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
	writeTitle(w, v.Module+" @ "+v.Version, styled)
	fmt.Fprintf(w, "%s\n", v.Summary())
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
// A commit that moved the exported API says so in the counts the stats table
// reads the whole release in, so the one commit behind a removal can be picked
// out of a run of twenty. A commit that moved nothing exported leaves the cell
// empty rather than writing three zeroes, since a table of "+0/~0/-0" hides
// the rows that matter.
func writeCommits(w io.Writer, v verdict, styled bool, wrap int) {
	counts := commitCounts(v, styled)

	headers := []string{"Commit", "Subject"}
	widths := []int{shortHashWidth(v.Commits), 0}
	if counts != nil {
		headers = []string{"Commit", "API", "Subject"}
		widths = []int{shortHashWidth(v.Commits), columnWidth("API", slices.Collect(maps.Values(counts))), 0}
	}
	subject := cellWidth(wrap, widths)

	rows := make([][]string, 0, len(v.Commits))
	for _, commit := range v.Commits {
		row := []string{commitLink(v, commit.Hash, styled)}
		if counts != nil {
			row = append(row, counts[commit.Hash])
		}
		rows = append(rows, append(row, fold(commit.Subject, subject)))
	}
	writeSimpleTable(w, headers, rows, styled)
}

// commitCounts renders the API column of the commit table, keyed on the short
// hash, and returns nil when no commit of the range moved the exported API:
// there is nothing a column of empty cells adds.
func commitCounts(v verdict, styled bool) map[string]string {
	counts := make(map[string]string, len(v.Commits))
	for _, commit := range v.Commits {
		diff, ok := v.CommitAPI[commit.Hash]
		if !ok || diff.Skipped != "" {
			continue
		}
		if len(diff.Added)+len(diff.Changed)+len(diff.Removed) == 0 {
			continue
		}
		counts[commit.Hash] = symbolCounts(diff, styled)
	}
	if len(counts) == 0 {
		return nil
	}
	return counts
}

// symbolCounts renders what a commit did to the exported API as the triple the
// stats table counts a release in: what it added, what it reshaped, what it
// took away.
func symbolCounts(diff apiDiff, styled bool) string {
	return colorLines("+"+strconv.Itoa(len(diff.Added)), components.ColorGreen, styled) +
		"/" + colorLines("~"+strconv.Itoa(len(diff.Changed)), components.ColorAmber, styled) +
		"/" + colorLines("-"+strconv.Itoa(len(diff.Removed)), components.ColorRed, styled)
}

// commitLink renders a commit hash the way the tables name it: a link into the
// repository in markdown, and a coloured hash on a terminal.
func commitLink(v verdict, hash string, styled bool) string {
	if styled {
		return colorLines(hash, components.ColorTeal, true)
	}
	if v.RepoURL == "" {
		return "`" + hash + "`"
	}
	return fmt.Sprintf("[`%s`](%s/commit/%s)", hash, v.RepoURL, hash)
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

// symbolEntry is one symbol as a row of the API table.
type symbolEntry struct {
	category string
	pkg      string
	text     string

	// commits are the short hashes of the commits that introduced or moved
	// the symbol, oldest first.
	commits []string
}

// symbolRows returns one row per symbol, in the order the report reads the
// categories: what the release adds first, what it takes away last.
//
// The category names only the first row of its group and the rows after it
// leave the column empty. The table draws no rule between rows, so a group
// reads as one block under its heading.
//
// A module holding more than one package gains a column naming which one a
// symbol belongs to, without which "const Name" three times over says nothing.
// The symbols of a package are gathered together and only the first of them
// names it, the same way the data model table reads.
//
// A range read commit by commit gains a column naming the commits behind each
// symbol, which is what turns a removal into something to go and read.
func symbolRows(v verdict, styled bool, wrap int) ([]string, [][]string) {
	touched := v.commitsBySymbol()

	var entries []symbolEntry
	for _, symbol := range v.API.Added {
		entries = append(entries, symbolEntry{"Added", symbol.Package, symbol.String(), touched[symbol.Key]})
	}
	for _, change := range v.API.Changed {
		entries = append(entries, symbolEntry{"Changed", change.Package, "Before: " + change.Old + "\nAfter: " + change.New, touched[change.Key]})
	}
	for _, symbol := range collapseRemovedMethods(v.API.Removed) {
		entries = append(entries, symbolEntry{"Removed", symbol.Package, symbol.String(), touched[symbol.Key]})
	}
	if len(entries) == 0 {
		return nil, nil
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
		groupByPackage(entries)
	}

	hashes := make([][]string, 0, len(entries))
	for _, entry := range entries {
		hashes = append(hashes, entry.commits)
	}
	commits := commitCells(v, hashes, styled)
	if commits != nil {
		headers = append(headers, "Commits")
		widths = append(widths, columnWidth("Commits", commits))
	}
	symbolWidth := cellWidth(wrap, append(widths, 0))

	var (
		rows                  [][]string
		lastCategory, lastPkg string
	)
	for i, entry := range entries {
		// A new category opens a group and names itself. The package is written
		// again under it, since a group opening on an empty cell says nothing.
		category := ""
		if entry.category != lastCategory {
			category = colorLines(entry.category, changeColor(entry.category), styled)
			lastCategory, lastPkg = entry.category, ""
		}

		row := []string{category}
		if packages {
			pkg := entry.pkg
			if pkg == lastPkg {
				pkg = ""
			}
			lastPkg = entry.pkg
			row = append(row, colorLines(pkg, components.ColorSeparator, styled))
		}
		row = append(row, fold(entry.text, symbolWidth))
		if commits != nil {
			row = append(row, commits[i])
		}
		rows = append(rows, row)
	}
	return headers, rows
}

// groupByPackage gathers the symbols of a package together within their
// category, so a column naming the package can leave the repeats empty. The
// order within a package is the one the comparison reported.
func groupByPackage(entries []symbolEntry) {
	rank := map[string]int{"Added": 0, "Changed": 1, "Removed": 2}
	sort.SliceStable(entries, func(i, j int) bool {
		if a, b := rank[entries[i].category], rank[entries[j].category]; a != b {
			return a < b
		}
		return entries[i].pkg < entries[j].pkg
	})
}

// packageColumn shortens every entry to its path below the module and reports
// whether the entries span more than one package, which is when naming them is
// worth a column.
func packageColumn(v verdict, entries []symbolEntry) bool {
	seen := make(map[string]bool)
	for i, entry := range entries {
		pkg := shortPackage(v.Module, entry.pkg)
		entries[i].pkg = pkg
		seen[pkg] = true
	}
	return len(seen) > 1
}

// shortPackage names a package the way the tables refer to it, as its import
// path below the module, written from the module root down: "/model" is the
// model package of this module and not some other one, and the package at the
// root of the module is "/".
//
// The name alone is not enough to go on. A module holding a model package
// under two directories has two of them, both named "model", and a column
// saying so twice tells a reader nothing about which is which.
//
// A package that is not below the module is named by its import path, which is
// the only name it has here.
func shortPackage(module, pkg string) string {
	if pkg == module {
		return "/"
	}
	if rel := strings.TrimPrefix(pkg, module+"/"); rel != pkg {
		return "/" + rel
	}
	return pkg
}

// fold breaks a line to a width, and leaves it alone when there is no width to
// break it to.
func fold(line string, width int) string {
	if width <= 0 {
		return line
	}
	return ansi.Wrap(line, width, "")
}

// changeColor returns the colour a category of change is written in: green for
// what a release adds, amber for what it reshapes, red for what it takes away.
func changeColor(category string) string {
	switch strings.ToLower(category) {
	case fieldAdded:
		return components.ColorGreen
	case fieldRemoved:
		return components.ColorRed
	}
	return components.ColorAmber
}

// newTypeMark flags a type the release introduces, so a field of a new type
// reads apart from one added to a type that was already there. It is the mark
// the dependency matrix uses, and for the same reason: something that is there
// now and was not before.
const newTypeMark = dependencyMark

// fieldEntry is one exported field as a row of the data model table.
type fieldEntry struct {
	category string
	pkg      string
	typeName string

	// newType reports that the type itself is new, rather than one that was
	// already there gaining a field.
	newType bool

	text string

	// commits are the short hashes of the commits that introduced or moved
	// the field, oldest first.
	commits []string
}

// writeDataModel writes what became of the exported fields of every type the
// release touches, which is the shape of the data a consumer reads and writes.
//
// It is one table rather than one per type: a release touching a dozen types
// otherwise writes a dozen tables that line up with none of the others.
func writeDataModel(w io.Writer, v verdict, styled bool, wrap int) {
	headers, rows := dataModelRows(v, styled, wrap)
	if len(rows) == 0 {
		return
	}
	writeHeading(w, "Data model "+v.Range(), styled)
	writeSimpleTable(w, headers, rows, styled)
	writeGap(w, styled)
}

// dataModelRows returns one row per field, in the order the report reads the
// categories: what the release adds first, what it takes away last.
//
// A cell repeating the one above it is left empty, so a type reads as one block
// of its fields. The category starts a group and the package and type are
// restated under it, since a group opening on three empty cells says nothing.
func dataModelRows(v verdict, styled bool, wrap int) ([]string, [][]string) {
	entries := dataModelEntries(v)
	if len(entries) == 0 {
		return nil, nil
	}

	headers := []string{"Change", "Package", "Type", "Field"}
	widths := []int{len("Removed"), len("Package"), len("Type")}
	for _, entry := range entries {
		widths[1] = max(widths[1], len(entry.pkg))
		widths[2] = max(widths[2], len(entry.typeName)+len(" "+newTypeMark))
	}

	hashes := make([][]string, 0, len(entries))
	for _, entry := range entries {
		hashes = append(hashes, entry.commits)
	}
	commits := commitCells(v, hashes, styled)
	if commits != nil {
		headers = append(headers, "Commits")
		widths = append(widths, columnWidth("Commits", commits))
	}
	fieldWidth := cellWidth(wrap, append(widths, 0))

	var (
		rows                            [][]string
		lastCategory, lastPkg, lastType string
	)
	for i, entry := range entries {
		// A new category opens a group, and names itself. The package and type
		// are written again under it, however far the group above them reached,
		// since a group opening on empty cells says nothing.
		category := ""
		if entry.category != lastCategory {
			category = entry.category
			lastCategory, lastPkg, lastType = entry.category, "", ""
		}

		row := dataModelRow(entry, category, styled, fieldWidth)
		if entry.pkg == lastPkg {
			row[1] = ""
			if entry.typeName == lastType {
				row[2] = ""
			}
		}
		if commits != nil {
			row = append(row, commits[i])
		}

		lastPkg, lastType = entry.pkg, entry.typeName
		rows = append(rows, row)
	}
	return headers, rows
}

// dataModelRow renders one field as the cells of a row, with the category named
// only when it opens a group.
func dataModelRow(entry fieldEntry, category string, styled bool, width int) []string {
	if category != "" {
		category = colorLines(categoryName(category), changeColor(category), styled)
	}

	name := entry.typeName
	if entry.newType {
		name += " " + colorLines(newTypeMark, components.ColorGreen, styled)
	}

	return []string{
		category,
		colorLines(entry.pkg, components.ColorSeparator, styled),
		name,
		fold(entry.text, width),
	}
}

// dataModelEntries returns every field the release moved, the added ones first,
// and within a category ordered by package, type and name.
//
// The fields of a type the release adds are read as additions of their own, so
// a reader sees the shape it declares rather than only that it exists. The
// unexported members are not there to see: they are nobody's promise to keep.
func dataModelEntries(v verdict) []fieldEntry {
	touched := v.commitsByField()

	var entries []fieldEntry
	add := func(key, pkg, typeName string, newType bool, change apiFieldChange) {
		entries = append(entries, fieldEntry{
			category: change.Change,
			pkg:      shortPackage(v.Module, pkg),
			typeName: typeName,
			newType:  newType,
			text:     fieldText(change),
			commits:  touched[fieldKey(key, change.Name)],
		})
	}

	for _, symbol := range addedTypes(v.API) {
		for _, field := range symbol.Fields {
			add(symbol.Key, symbol.Package, symbol.Name, true, apiFieldChange{
				Name: field.Name, Change: fieldAdded, New: &field,
			})
		}
	}
	for _, want := range []string{fieldAdded, fieldChanged, fieldRemoved} {
		for _, change := range v.API.Types {
			for _, field := range change.Fields {
				if field.Change == want {
					add(change.Key, change.Package, change.Name, false, field)
				}
			}
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.category != b.category {
			return categoryOrder(a.category) < categoryOrder(b.category)
		}
		if a.pkg != b.pkg {
			return a.pkg < b.pkg
		}
		if a.typeName != b.typeName {
			return a.typeName < b.typeName
		}
		return a.text < b.text
	})
	return entries
}

// categoryOrder ranks the categories the way the report reads them, which is
// what a release adds first and what it takes away last.
func categoryOrder(category string) int {
	switch category {
	case fieldAdded:
		return 0
	case fieldChanged:
		return 1
	}
	return 2
}

// categoryName is the change a field underwent, as a table column names it.
func categoryName(category string) string {
	switch category {
	case fieldAdded:
		return "Added"
	case fieldChanged:
		return "Changed"
	case fieldRemoved:
		return "Removed"
	}
	return category
}

// addedTypes returns the types the release adds that declare an exported field,
// which is what there is to write a shape for. A type with none, such as a func
// type or a named string, is already said by its row in the API table.
func addedTypes(diff apiDiff) []apiSymbol {
	var types []apiSymbol
	for _, symbol := range diff.Added {
		if symbol.Kind == "type" && len(symbol.Fields) > 0 {
			types = append(types, symbol)
		}
	}
	return types
}

// fieldText renders a field as the table lists it: its name, and the shape it
// has or the two shapes it moved between.
//
// The name leads, since it has no column of its own, and is written once for a
// field that moved: a field is matched to the one it was by name, so the name
// is the one thing that cannot have changed.
//
// A tag is part of the shape, since it is what a stored document decodes
// through, and is written alongside the type whenever either side carries one.
func fieldText(change apiFieldChange) string {
	switch {
	case change.Old != nil && change.New != nil:
		return fieldLabel(change.Name, fieldShape(*change.Old)) + " -> " + fieldShape(*change.New)
	case change.New != nil:
		return fieldLabel(change.Name, fieldShape(*change.New))
	case change.Old != nil:
		return fieldLabel(change.Name, fieldShape(*change.Old))
	}
	return change.Name
}

// fieldLabel writes the name in front of a shape, unless the shape opens on it
// already. An interface method carries its name in the signature it is recorded
// under, as "Put (key string) error", where the type of a struct field does
// not.
//
// The parameter list is what the name is recognised by, not the name alone: a
// field is often typed after itself, and "Mode Mode" is a field named Mode of
// type Mode rather than a name written twice.
func fieldLabel(name, shape string) string {
	if strings.HasPrefix(shape, name+" (") {
		return shape
	}
	return name + " " + shape
}

// fieldShape renders one side of a field, which is its type and the tag it
// carries.
func fieldShape(field apiField) string {
	if field.Tag == "" {
		return field.Type
	}
	return field.Type + " `" + field.Tag + "`"
}

// collapseRemovedMethods drops the methods and receiver-bound declarations of
// a type that is itself removed: the type row already says everything its
// methods would, since a type cannot go away and leave them behind.
func collapseRemovedMethods(symbols []apiSymbol) []apiSymbol {
	types := make(map[string]bool)
	for _, symbol := range symbols {
		if symbol.Kind == "type" {
			types[symbol.Package+"\x00"+symbol.Name] = true
		}
	}
	kept := make([]apiSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		if receiver := receiverType(symbol.Name); receiver != "" && types[symbol.Package+"\x00"+receiver] {
			continue
		}
		kept = append(kept, symbol)
	}
	return kept
}

// receiverType returns the type a qualified name hangs off ("Disk.Save" is
// hung off "Disk"), or an empty string for a package level name.
func receiverType(name string) string {
	receiver, _, found := strings.Cut(name, ".")
	if !found {
		return ""
	}
	return receiver
}
