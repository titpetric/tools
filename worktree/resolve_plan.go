package main

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// resolvePlan is the resolution of one module: the state it is in, the version
// it ends up at, and what has to happen to get it there.
type resolvePlan struct {
	// Module is the go module path, Path the directory holding it. A
	// repository that is not a go module is named by its path.
	Module string
	Path   string

	// GoModule reports whether the directory holds a go.mod. Without one
	// there are no requirements to update and no API to read, so the module
	// is resolved on its git state alone.
	GoModule bool

	// Latest is the version of the newest release, empty when the module has
	// none. TagPrefix is what its git tags carry in front of that version,
	// which is "<subdir>/" for a module nested in a larger repository.
	Latest    string
	TagPrefix string

	// Ahead counts the commits made to the module since Latest.
	Ahead int

	// Dirty holds the working tree changes that resolve does not commit
	// itself, as "<status> <path>". A module reaching its dirty check with
	// any of these stops the run.
	Dirty []string

	// Pins are the workspace requirements to move, each naming the version
	// its dependency ends up at.
	Pins []requireInfo

	// API is the exported symbol difference since Latest, which is what
	// chooses between a patch and a minor release.
	API apiDiff

	// Release is releasePatch, releaseMinor, or empty when the module is not
	// tagged by this run.
	Release string

	// Next is the tag the release step creates.
	Next string

	// GoFrom is the go directive the module declares now, GoTo the one this
	// run sets it to, empty when it is already at the highest the selection
	// declares, and GoSince the one it declared at its latest tag.
	GoFrom  string
	GoTo    string
	GoSince string

	// Conditional reports that the module is only released if the update
	// rewrites go.mod or go.sum. It has no commits of its own and nothing in
	// the workspace to move, so only its outside dependencies can earn it a
	// release, and whether they have is not known until go get has run.
	Conditional bool

	// Skip records why the module needs no work, and is empty when it does.
	Skip string
}

// resolveOrder returns mods ordered so that a module follows every module it
// depends on, together with the ones left over in a dependency cycle.
//
// Dependencies outside mods are ignored: resolve works on the modules it was
// asked for, and a module it was not asked for is never bumped, so it cannot
// come to hold a version that does not exist yet.
func resolveOrder(mods []string, uses map[string][]string) (order, cycles []string) {
	selected := make(map[string]bool, len(mods))
	for _, mod := range mods {
		selected[mod] = true
	}

	// waiting counts the dependencies a module still has in the order, and
	// blocks maps a dependency to the modules waiting on it.
	waiting := make(map[string]int, len(mods))
	blocks := make(map[string][]string, len(mods))
	for _, mod := range mods {
		for _, dep := range uses[mod] {
			if !selected[dep] || dep == mod {
				continue
			}
			waiting[mod]++
			blocks[dep] = append(blocks[dep], mod)
		}
	}

	var ready []string
	for _, mod := range mods {
		if waiting[mod] == 0 {
			ready = append(ready, mod)
		}
	}
	sort.Strings(ready)

	for len(ready) > 0 {
		mod := ready[0]
		ready = ready[1:]
		order = append(order, mod)

		var freed []string
		for _, dependant := range blocks[mod] {
			waiting[dependant]--
			if waiting[dependant] == 0 {
				freed = append(freed, dependant)
			}
		}
		// The freed modules are merged in sorted, so the order of two
		// modules that could equally well come next never depends on the
		// order the map handed them out in.
		ready = append(ready, freed...)
		sort.Strings(ready)
	}

	for _, mod := range mods {
		if waiting[mod] > 0 {
			cycles = append(cycles, mod)
		}
	}
	return order, cycles
}

// planResolve works out what each module needs, walking them in dependency
// order so that a module already knows the version its dependencies end up at
// by the time its own requirements are read.
func planResolve(modules []moduleInfo, refs versionRefs) (plans []resolvePlan, cycles []string) {
	var (
		names = make([]string, 0, len(modules))
		dirs  = make(map[string]string, len(modules))
		uses  = make(map[string][]string, len(modules))
	)
	for _, module := range modules {
		names = append(names, module.Name)
		dirs[module.Name] = module.Path
		uses[module.Name] = module.Uses
	}

	order, cycles := resolveOrder(names, uses)

	// Every module is brought up to the highest go directive the selection
	// declares, so a workspace resolved together stays on one language
	// version rather than drifting a release apart.
	goTarget := ""
	if latest, found := latestGoVersion(modules); found {
		goTarget = fmt.Sprintf("%d.%d", latest.Major, latest.Minor)
	}

	// targets holds the version each module ends up at, which is the tag this
	// run creates for it when it gets one, and the tag it already carries
	// otherwise.
	targets := make(map[string]string, len(order))

	for _, name := range order {
		dir := dirs[name]
		plan := resolvePlan{Module: name, Path: dir, GoModule: isGoModule(dir)}

		tags, prefix, err := moduleTags(dir)
		plan.TagPrefix = prefix
		if err == nil {
			if latest, found := LatestRelease(tags); found {
				plan.Latest = latest.String()
				plan.Ahead = commitsSinceTag(dir, prefix+plan.Latest)
			}
		}
		targets[name] = plan.Latest

		plan.Pins = resolvePins(name, uses[name], refs, targets)
		plan.Dirty = dirtyFiles(dir)

		if plan.GoModule {
			plan.GoFrom = readGoVersion(dir)
			if goTarget != "" && goVersionOutdated(plan.GoFrom, mustParseGoDirective(goTarget)) {
				plan.GoTo = goTarget
			}
			if plan.Latest != "" {
				plan.GoSince = goVersionAt(dir, prefix+plan.Latest)
			}
		}

		// A released go module is always offered the update, even with no
		// commits and nothing in the workspace to move: its dependencies
		// outside the workspace can still have moved, and go get -u is the
		// only way to find out. Whether that earns a release is only known
		// once go.mod and go.sum have been rewritten, so the release is
		// conditional on them changing.
		plan.Conditional = plan.GoModule && plan.Latest != "" && plan.Ahead == 0 &&
			len(plan.Pins) == 0 && plan.GoTo == ""

		switch {
		case !plan.GoModule && plan.Ahead == 0:
			plan.Skip = "up to date"
		case plan.Latest == "" && len(plan.Pins) == 0:
			// An untagged module is not given a first release by resolve;
			// there is nothing to compare its API against and nothing
			// downstream can pin to it.
			plan.Skip = "no release tag, nothing to update"
		}
		if plan.Skip != "" {
			plans = append(plans, plan)
			continue
		}

		if plan.Latest != "" {
			plan.API = apiDiff{Skipped: "not a go module"}
			if plan.GoModule {
				plan.API = apiDiffSinceTag(dir, prefix+plan.Latest)
			}
			plan.Release = releaseKind(plan)

			next, _, err := nextRelease(tags, plan.Release)
			if err == nil {
				plan.Next = next.String()
				// A conditional module is assumed to release, so a module
				// depending on it asks for the version it would get. What
				// each module actually ends at is read back under --apply.
				targets[name] = plan.Next
			}
		}

		plans = append(plans, plan)
	}
	return plans, cycles
}

// releaseKind returns the release a module has earned.
//
// Taking exported API away costs a minor, and so does moving to another go
// release series, whether the go directive was raised by hand since the tag or
// is being raised by this run: the module stops building for anyone on the
// older toolchain, which is as breaking as a symbol going away. A point
// release of the same series, 1.27 to 1.27.1, changes nothing for a consumer.
//
// Everything else, including a dependency update that only rewrites go.mod and
// go.sum, is a patch.
func releaseKind(plan resolvePlan) string {
	switch {
	case plan.API.Breaking:
		return releaseMinor
	case plan.GoTo != "" && goSeriesChanged(plan.GoFrom, plan.GoTo):
		return releaseMinor
	case plan.GoSince != "" && goSeriesChanged(plan.GoSince, plan.GoFrom):
		return releaseMinor
	}
	return releasePatch
}

// mustParseGoDirective parses a directive this program built itself.
func mustParseGoDirective(directive string) Version {
	v, _ := ParseGoDirective(directive)
	return v
}

// resolvePins returns the workspace requirements of a module that name a
// version other than the one their dependency ends up at. It is the same
// comparison staleRequires makes for -u, against the versions this run creates
// rather than against the tags that exist now.
func resolvePins(module string, uses []string, refs versionRefs, targets map[string]string) []requireInfo {
	var pins []requireInfo
	for _, dep := range uses {
		target := targets[dep]
		if target == "" || refs[module][dep] == target {
			continue
		}
		pins = append(pins, requireInfo{path: dep, version: target})
	}
	sort.Slice(pins, func(i, j int) bool { return pins[i].path < pins[j].path })
	return pins
}

// dirtyFiles returns the working tree changes of the module in dir, as
// "<status> <path>", leaving out the go.mod and go.sum that resolve commits
// itself. Without --apply this is the prediction of the state the module is in
// once that commit is made.
func dirtyFiles(dir string) []string {
	root, rel, err := repoPaths(dir)
	if err != nil {
		return nil
	}

	args := []string{"status", "--porcelain"}
	if rel != "." {
		args = append(args, "--", rel)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var files []string
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if len(line) < 4 {
			continue
		}
		status, path := strings.TrimSpace(line[:2]), line[3:]
		if rel != "." {
			path = strings.TrimPrefix(path, rel+"/")
		}
		if path == "go.mod" || path == "go.sum" {
			continue
		}
		files = append(files, status+" "+path)
	}
	return files
}
