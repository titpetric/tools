package main

import (
	"path/filepath"
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

	// Visibility is what each package of the working tree declares, split
	// exported against internal. It describes the tree as it stands rather
	// than the range, and is empty when it could not be read.
	Visibility visibilityReport

	// CommitAPI is what each of the commits did to the exported API on its
	// own, keyed on its short hash. It is nil for a range that was not read
	// commit by commit, which is one the tool could not scan.
	CommitAPI map[string]apiDiff

	// GoBefore and GoAfter are the go directive at each of the two
	// revisions. Moving to another release series costs a minor.
	GoBefore string
	GoAfter  string

	// Scope is the package pattern the report was narrowed to, empty for the
	// whole module, which is the implicit "./...".
	Scope string

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
	v.Visibility = readVisibility(dir)

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
