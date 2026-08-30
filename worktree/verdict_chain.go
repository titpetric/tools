package main

import (
	"fmt"
	"io"
	"strconv"

	"github.com/titpetric/tools/worktree/components"
)

// versionRange is one section of a release chain, as the two versions it is
// measured between. An empty From is the start of history, which is what the
// first tag is measured from, and an empty To is the working tree.
type versionRange struct {
	From string
	To   string
}

// series is the major.minor pair a release belongs to. Releases sharing one are
// the same line of work, and the chain reports the move from one to the next.
type series struct {
	Major int
	Minor int
}

// seriesOf returns the series a version belongs to.
func seriesOf(v Version) series {
	return series{Major: v.Major, Minor: v.Minor}
}

// seriesOpeners returns the lowest release of each major.minor series, in
// ascending order. That is usually the .0 of the series, and is whatever came
// first when the .0 was never tagged.
func seriesOpeners(tags []string) []Version {
	var (
		openers []Version
		seen    = make(map[series]bool)
	)
	for _, release := range Releases(tags) {
		s := seriesOf(release)
		if seen[s] {
			continue
		}
		seen[s] = true
		openers = append(openers, release)
	}
	return openers
}

// releaseChain returns the ranges a full report covers, newest first.
//
// The chain is drawn between series rather than between every tag: a patch
// release is not a change of direction, and the commits it carries are still
// reported, inside the wider section spanning it. What the chain holds is
//
//   - the first release, measured from the first commit, since there is no
//     earlier release to compare it against,
//   - each series opener against the opener of the series below it, so the move
//     from 0.1 to 0.2 is 0.1.0..0.2.0 and not 0.1.1..0.2.0,
//   - the latest release against the opener of its own series, when the two are
//     not the same release. The patches of every earlier series are covered by
//     the section that spans them; those of the latest series are covered by
//     nothing else,
//   - the working tree, when there are commits on top of the latest tag.
//
// A verbose chain reports every release instead, each against the one before
// it, which is the same history at the granularity it was tagged at.
//
// The from and upTo versions bound the chain at either end, and are empty for a
// chain running the whole history to the latest release. A chain bounded above
// never reports the working tree, which is past its end; one bounded below
// starts at the release named rather than at the first commit.
func releaseChain(tags []string, ahead, verbose bool, from, upTo string) []versionRange {
	releases, steps := Releases(tags), seriesOpeners(tags)
	if verbose {
		steps = releases
	}
	if bound, ok := ParseVersion(upTo); ok {
		releases, steps = releasesAtOrBelow(releases, bound), releasesAtOrBelow(steps, bound)
		ahead = false
	}

	// A chain bounded below is measured from the release naming the bound, so
	// the release below it is where the report starts rather than the first
	// commit.
	start, bounded := ParseVersion(from)
	if bounded {
		releases, steps = releasesAtOrAbove(releases, start), releasesAtOrAbove(steps, start)
	}

	if len(releases) == 0 {
		if bounded {
			return nil
		}
		// Nothing was released below the end of the chain, so the whole history
		// is the one release it holds.
		return []versionRange{{To: upTo}}
	}

	var ranges []versionRange
	if !bounded {
		// The first release, measured from the first commit.
		ranges = append(ranges, versionRange{To: releases[0].String()})
	}
	for i := 1; i < len(steps); i++ {
		ranges = append(ranges, versionRange{From: steps[i-1].String(), To: steps[i].String()})
	}

	// The latest release, when the steps above stopped short of it. That is the
	// case for a patch release of the newest series, whose work no other
	// section covers.
	latest := releases[len(releases)-1]
	if last := steps[len(steps)-1]; Compare(last, latest) < 0 {
		ranges = append(ranges, versionRange{From: last.String(), To: latest.String()})
	}

	// The work on top of the latest tag, which is the release it has earned.
	if ahead {
		ranges = append(ranges, versionRange{From: latest.String()})
	}

	return reversed(ranges)
}

// releasesAtOrBelow returns the releases at or below a version, which is how a
// chain is bounded by the release it is asked to stop at.
func releasesAtOrBelow(releases []Version, bound Version) []Version {
	var below []Version
	for _, release := range releases {
		if Compare(release, bound) <= 0 {
			below = append(below, release)
		}
	}
	return below
}

// releasesAtOrAbove returns the releases at or above a version, which is how a
// chain is bounded by the release it is asked to start from.
func releasesAtOrAbove(releases []Version, bound Version) []Version {
	var above []Version
	for _, release := range releases {
		if Compare(release, bound) >= 0 {
			above = append(above, release)
		}
	}
	return above
}

// reversed returns the ranges newest first, which is the order a changelog is
// read in.
func reversed(ranges []versionRange) []versionRange {
	out := make([]versionRange, 0, len(ranges))
	for i := len(ranges) - 1; i >= 0; i-- {
		out = append(out, ranges[i])
	}
	return out
}

// readVerdicts reports on every release the module in dir has made, one verdict
// per section of its release chain, newest first.
//
// Every revision is read through one model cache, so a release that ends one
// section and starts the next is unpacked and modelled once rather than twice.
// The cached flag is whether that cache is backed by the one on disk, which
// carries the commits of a run into the next one, and --no-cache turns off.
func readVerdicts(dir string, verbose bool, from, upTo string, cached bool) ([]verdict, error) {
	tags, prefix, err := moduleTags(dir)
	if err != nil {
		return nil, err
	}

	ahead := false
	if latest, found := LatestRelease(tags); found {
		ahead = commitsSinceTag(dir, prefix+latest.String()) > 0
	}

	chain := releaseChain(tags, ahead, verbose, from, upTo)
	if len(chain) == 0 {
		return nil, fmt.Errorf("no release falls in the range asked for")
	}

	models, err := newAPIModels(cached)
	if err != nil {
		return nil, err
	}
	defer func() { _ = models.Close() }()

	base := verdict{Module: moduleName(dir), RepoURL: repoURL(dir)}

	var verdicts []verdict
	for _, r := range chain {
		v, err := base.report(dir, tags, prefix, r.From, r.To, models)
		if err != nil {
			return nil, err
		}
		verdicts = append(verdicts, v)
	}
	return verdicts, nil
}

// renderVerdicts writes every section of a release chain, newest first, each
// the report a single verdict writes, separated by a blank line. On a terminal
// that lands under the blank line the report before it ends on, so one release
// stands further from the next than the tables inside it stand from each other.
func renderVerdicts(w io.Writer, verdicts []verdict, styled bool) {
	for i, v := range verdicts {
		if i > 0 {
			fmt.Fprintln(w)
		}
		renderVerdict(w, v, styled)
	}
}

// renderVerdictStats writes the whole run as one table of counts, a row per
// release, newest first. It is the report with the analysis collapsed: what
// each release did to the API and to the data model, without the symbols
// behind it.
func renderVerdictStats(w io.Writer, verdicts []verdict, styled bool) {
	if len(verdicts) == 0 {
		return
	}

	writeTitle(w, verdicts[0].Module, styled)

	headers := []string{
		"Version", "Since", "Commits",
		"Symbols +", "Symbols ~", "Symbols -",
		"Fields +", "Fields ~", "Fields -",
	}

	rows := make([][]string, 0, len(verdicts))
	for _, v := range verdicts {
		added, changed, removed := v.API.FieldCounts()
		rows = append(rows, []string{
			v.Version,
			v.Since,
			strconv.Itoa(len(v.Commits)),
			count(len(v.API.Added), components.ColorGreen, styled),
			count(len(v.API.Changed), components.ColorAmber, styled),
			count(len(v.API.Removed), components.ColorRed, styled),
			count(added, components.ColorGreen, styled),
			count(changed, components.ColorAmber, styled),
			count(removed, components.ColorRed, styled),
		})
	}
	writeSimpleTable(w, headers, rows, styled)
	writeGap(w, styled)
}

// count renders one cell of the stats table. A zero is left grey, so the
// releases that did something stand out from the ones that did not.
func count(n int, color string, styled bool) string {
	if n == 0 {
		return colorLines("0", components.ColorSeparator, styled)
	}
	return colorLines(strconv.Itoa(n), color, styled)
}
