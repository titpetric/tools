package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// verdict is the release a module has earned, or the one it last had: the
// version the report describes it at, the commits that went into it, and what became
// of its exported API.
type verdict struct {
	// Module names the module, which is its go module path when it has a
	// go.mod and its directory otherwise.
	Module string

	// Version is the release the report describes, which is the one this
	// module moves to when it sits behind its latest tag and the latest tag
	// itself otherwise.
	Version string

	// Released reports that Version is a tag that exists, so the report covers
	// what went into it and leaves out what a release would take.
	Released bool

	// Since is the version the comparison measures from, empty when no such
	// version exists.
	Since string

	// Release is releasePatch or releaseMinor, and is empty for a release
	// that already happened.
	Release string

	// Commits are the commits between Since and Version, newest first.
	Commits []commitLog

	// API is the exported symbol difference between the two.
	API apiDiff

	// Visibility is what each package of the working tree declares, split
	// exported against internal. It describes the tree as it stands, and
	// leaves the range out; it stays empty when the read fails.
	Visibility visibilityReport

	// CommitAPI is what each of the commits did to the exported API on its
	// own, keyed on its short hash. It holds nil for a range the tool did not
	// read commit by commit, because the scan failed.
	CommitAPI map[string]apiDiff

	// GoBefore and GoAfter are the go directive at each of the two
	// revisions. Moving to another release series costs a minor.
	GoBefore string
	GoAfter  string

	// Scope is the package pattern the report narrows to, empty for the
	// whole module, which is the implicit "./...".
	Scope string

	// RepoURL is the address the report links commits into, empty when the module
	// has no origin to derive one from.
	RepoURL string
}

// readVerdict works out the release the module in dir has earned, or reports
// the one it last had.
//
// For a module with commits since its latest tag, readVerdict measures from
// that tag to the working tree, and bumps the version as a minor when the
// release takes exported API away and as a patch otherwise. A module level with
// its tag has nothing to propose, so readVerdict describes the last release,
// measured from the release before it. A module with no tag at all gets a first
// release, measured from the start of history, where the report lists
// everything it exports as added.
// The --from and --to flags name the two revisions outright, so a report can
// cover a range away from the current checkout.
//
// The detailed flag reads every commit of the range on its own, which fills in
// the commit table and the commits behind each symbol. Without it the report
// compares the two ends alone: a commit is immutable, and modelling each one
// costs a splint extraction, and a range of a few hundred commits takes
// minutes. -v turns it on.
//
// The cached flag says whether the cache keeps the models of the commits it
// reads between runs; --no-cache turns that off.
func readVerdict(dir, from, to string, detailed, cached bool) (verdict, error) {
	tags, prefix, err := moduleTags(dir)
	if err != nil {
		return verdict{}, err
	}
	reportTags(tags)

	v := verdict{Module: moduleName(dir), RepoURL: repoURL(dir)}

	// One cache holds the range and every commit inside it, so a commit that
	// ends one step and starts the next gets one model.
	models, err := newAPIModels(cached)
	if err != nil {
		return verdict{}, err
	}
	defer func() { _ = models.Close() }()

	if from != "" || to != "" {
		return v.between(dir, tags, prefix, from, to, detailed, models)
	}

	latest, found := LatestRelease(tags)
	if !found {
		return v.report(dir, tags, prefix, "", "", detailed, models)
	}

	if ahead := commitsSinceTag(dir, prefix+latest.String()); ahead > 0 {
		return v.report(dir, tags, prefix, latest.String(), "", detailed, models)
	}

	// Level with the tag: the release to report on is the existing one,
	// measured from the release before it, or from the start of history when it
	// is the first one. Both are empty here, so from is the one or the other.
	if previous, ok := PreviousRelease(tags); ok {
		from = previous.String()
	}
	return v.report(dir, tags, prefix, from, latest.String(), detailed, models)
}

// reportTags writes the number of tags the run found, before it reads any
// revision. A run over a long range takes minutes, and this line says the
// repository was read and the range is being measured.
func reportTags(tags []string) {
	fmt.Fprintf(os.Stderr, "%d tags found\n", len(tags))
}

// between reports on the range the caller named. An empty from falls back to
// the release before the one named by to, and an empty to is the working tree.
// The report function reports the range it settles on.
func (v verdict) between(dir string, tags []string, prefix, from, to string, detailed bool, models *apiModels) (verdict, error) {
	if from == "" {
		// Measure from the revision before the requested one: the
		// release below it when it names one.
		if version, ok := ParseVersion(to); ok {
			if previous, found := PreviousRelease(releasesBelow(version, tags)); found {
				from = previous.String()
			}
		}
		if from == "" {
			if latest, found := LatestRelease(tags); found {
				from = latest.String()
			}
		}
	}

	return v.report(dir, tags, prefix, from, to, detailed, models)
}

// report fills in the verdict for a range whose two ends are already settled,
// where an empty from is the start of history and an empty to is the working
// tree. It fills in nothing for either end: a caller naming both, as the
// release chain does, gets the range it named.
//
// A range starting at the start of history has no earlier revision to measure
// against, so the report lists everything the module exports at the far end of
// it as added, the same as a first release.
//
// The report treats a "to" naming a release as that release; anything else is a
// proposal, since there is no tag to call it by.
//
// The models are the cache the comparison reads revisions through, and may be
// nil, in which case report reads the revisions for this range alone. The
// detailed flag reads each commit of the range on its own; without it the
// report compares the two ends and leaves the commit attribution out.
func (v verdict) report(dir string, tags []string, prefix, from, to string, detailed bool, models *apiModels) (verdict, error) {
	fromRef, toRef := taggedRef(from, prefix, tags), taggedRef(to, prefix, tags)

	v.Since = from
	v.Commits = commitLogBetween(dir, fromRef, toRef)
	v.API = compareRefs(dir, fromRef, toRef, models)
	if detailed {
		v.CommitAPI = scanCommits(dir, fromRef, v.Commits, models)
	}
	v.GoBefore, v.GoAfter = goVersionAt(dir, fromRef), goVersionAt(dir, toRef)
	v.Visibility = readVisibility(dir)

	if version, ok := ParseVersion(to); ok {
		v.Version = version.String()
		v.Released = true
		return v, nil
	}
	return v.propose(tags)
}

// taggedRef turns a version named on the command line into the tag the
// repository carries for it, and leaves anything else alone so a caller can
// name a commit or a branch just as well.
//
// The tag may differ in spelling from the requested version: with the v the
// command line left off, or under the subdirectory prefix of a nested module.
// taggedRef matches the version against the tags and does not reprint it, so
// "0.5.0" finds v0.5.0 and "v0.5.0" finds 0.5.0.
//
// taggedRef passes a version the repository has no tag for to git as it stands,
// and git reports it as an unreadable revision; nothing falls back to another
// revision silently.
func taggedRef(ref, prefix string, tags []string) string {
	version, ok := ParseVersion(ref)
	if !ok {
		return ref
	}
	for _, tag := range tags {
		if carried, ok := ParseVersion(tag); ok && Compare(carried, version) == 0 {
			return prefix + tag
		}
	}
	return prefix + ref
}

// releasesBelow returns the tags naming a release at or below version, so the
// caller can find the release before an existing one.
func releasesBelow(version Version, tags []string) []string {
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
// models are the cache compareRefs reads the revisions through, and may be nil for a
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
// scanCommits walks the commits oldest first, measuring each from the one
// before it, so the scans of a range add up to the difference across it: what
// one commit adds and the next takes away counts as an addition and a removal
// here, and as neither there. The commit under the oldest one is the revision
// the range starts from; for the start of history that is a module holding no
// packages, the same base the range itself compares against.
//
// The commits are those that touched the module, so scanCommits neither lists
// nor reads a commit elsewhere in the repository. Such a commit cannot move the
// API: the model comes from the module subtree alone.
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

// eachCommit calls fn for every scanned commit of the range, oldest first, the
// order the report names the commits behind a symbol in.
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

// fieldKey names one exported field of one type; the data model table lists a
// row per fieldKey.
func fieldKey(typeKey, field string) string {
	return typeKey + "." + field
}

// appendCommit adds a commit to the ones behind a symbol, unless the commit sits
// there already: a symbol that a single commit both reshapes and moves is that
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
