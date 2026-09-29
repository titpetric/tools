package main

import (
	"fmt"
	"strings"
)

// Summary states the verdict in one sentence, giving the reason behind it, and
// names the dependency movement after it when the release carries any.
func (v verdict) Summary() string {
	return v.summaryBase() + v.depNote()
}

// summaryBase is the verdict itself: the version, and what earned it.
func (v verdict) summaryBase() string {
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
	return plural(len(v.API.ExportedAdded()), "exported symbol is added", "exported symbols are added")
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
	if removed := len(v.API.ExportedRemoved()); removed > 0 {
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
