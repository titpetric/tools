package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/titpetric/tools/worktree/components"
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

// resolveRun collects the lines of one module's resolution cell, and performs
// the steps when --apply was given.
//
// Commands are run with their working directory set to the module they belong
// to, so no step ever changes the directory worktree itself is in. The table
// names that directory, which is why the steps do not.
type resolveRun struct {
	apply   bool
	verbose bool
	styled  bool

	// wrap is the width the cell is folded at, and is zero when the output
	// is not going to a terminal and folding it would only get in the way.
	wrap int

	// lines are the cell of the module being resolved.
	lines []string

	// actual maps a module to the version it ended at, which is only known
	// while performing a run and can differ from what the plan predicted.
	actual map[string]string
}

// setGo raises the go directive of a module to the version the run aligns on.
//
// A toolchain directive older than the new version leaves go.mod invalid, so
// it is dropped where there is one; go get and go mod tidy put a newer one
// back when they need it. This is what setGoVersion does for the --go flag,
// spelled as the commands that do it so the step reads as one.
func (r *resolveRun) setGo(plan resolvePlan) error {
	if toolchain := readToolchain(plan.Path); toolchain != "" && goVersionOutdated(toolchain, mustParseGoDirective(plan.GoTo)) {
		if err := r.run(plan.Path, "go", "mod", "edit", "-toolchain=none"); err != nil {
			return err
		}
	}
	return r.run(plan.Path, "go", "mod", "edit", "-go="+plan.GoTo)
}

// update runs one go get and tidies straight after it, so the requirement it
// replaced leaves go.mod and its checksums leave go.sum before the next one is
// asked for.
func (r *resolveRun) update(dir string, env []string, args ...string) error {
	if err := r.retry(dir, env, args...); err != nil {
		return err
	}
	return r.run(dir, "go", "mod", "tidy")
}

// add appends a colored line, folded to the width left for the cell.
func (r *resolveRun) add(color, format string, args ...any) {
	text := fmt.Sprintf(format, args...)
	if r.wrap > 0 {
		text = ansi.Wrap(text, r.wrap, "")
	}
	r.lines = append(r.lines, colorLines(text, color, r.styled))
}

// cell returns the lines collected for a module, and empties them for the next.
func (r *resolveRun) cell() string {
	cell := strings.Join(r.lines, "\n")
	r.lines = nil
	return cell
}

// resolve renders the plan for the selected modules, and performs it under
// --apply. The run stops at the first module whose working tree holds changes
// resolve did not make, since releasing it would tag work nobody reviewed, and
// every module after it would pin a version that never gets created.
//
// Modules with nothing to do are left out unless --all asks for them, which is
// what the flag means everywhere else in the tool.
func resolve(w io.Writer, modules []moduleInfo, refs versionRefs, opts *Options, styled bool) error {
	plans, cycles := planResolve(modules, refs)

	for _, mod := range cycles {
		fmt.Fprintln(w, colorLines("dependency cycle, not resolved: "+components.ShortPath(mod), components.ColorAmber, styled))
	}

	shown := plans
	if !opts.All {
		shown = nil
		for _, plan := range plans {
			if plan.Skip == "" {
				shown = append(shown, plan)
			}
		}
	}
	if len(shown) == 0 {
		fmt.Fprintln(w, colorLines("Nothing to resolve.", components.ColorGreen, styled))
		return nil
	}

	headers := []string{"Path", "Module", "Release", "Resolution"}
	widths := headerWidths(headers)
	for _, plan := range shown {
		widths[0] = max(widths[0], ansi.StringWidth(relPath(plan.Path)))
		widths[1] = max(widths[1], ansi.StringWidth(components.ShortPath(plan.Module)))
		widths[2] = max(widths[2], ansi.StringWidth(releaseCell(plan, false)))
	}

	run := &resolveRun{apply: opts.Apply, verbose: opts.Verbose, styled: styled}
	if styled {
		run.wrap = cellWidth(terminalWidth(w), widths)
	}

	table := newStreamTable(w, headers, widths, styled)
	defer table.close()

	var (
		stopped string
		failure error
	)
	for _, plan := range shown {
		table.start(relPath(plan.Path), components.ShortPath(plan.Module), releaseCell(plan, styled))

		switch {
		case stopped != "":
			run.add(components.ColorSeparator, "Not reached, %s.", stopped)
		case plan.Skip != "":
			run.add(components.ColorGreen, "%s.", plan.Skip)
		default:
			// A module that cannot be resolved stops the run the way a dirty
			// one does: every module after it would pin a version that never
			// gets created.
			reason, err := run.module(plan)
			if err != nil {
				reason = err.Error()
				failure = fmt.Errorf("resolve stopped at %s: %w", components.ShortPath(plan.Module), err)
			}
			if reason != "" {
				run.add(components.ColorRed, "Stopped: %s.", reason)
				stopped = "the run stopped at " + components.ShortPath(plan.Module)
			}
		}

		table.finish(run.cell())
	}
	return failure
}

// releaseCell renders the release column: the version a module is at, the
// commits it has taken since, and the version this run moves it to. The next
// version is green for a patch and amber for a minor, so what a release costs
// reads at a glance.
func releaseCell(plan resolvePlan, styled bool) string {
	if plan.Latest == "" {
		return colorLines("none", components.ColorSeparator, styled)
	}

	cell := colorLines(plan.Latest, components.ColorTeal, styled)
	if plan.Ahead > 0 {
		cell += " " + colorLines(fmt.Sprintf("(+%d)", plan.Ahead), components.ColorSeparator, styled)
	}
	if plan.Next != "" && plan.Next != plan.Latest {
		color := components.ColorGreen
		if plan.Release == releaseMinor {
			color = components.ColorAmber
		}
		cell += " " + colorLines("→ "+plan.Next, color, styled)
	}
	return cell
}

// terminalWidth returns the width of the terminal behind w, or zero when there
// is none to measure.
func terminalWidth(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return 0
	}
	width, _, err := term.GetSize(f.Fd())
	if err != nil {
		return 0
	}
	return width
}

// cellWidth returns the width left for the open last column of a table whose
// leading columns have the given widths. A terminal too narrow to fold into is
// reported as zero, which leaves the cell unfolded.
func cellWidth(terminal int, widths []int) int {
	if terminal <= 0 {
		return 0
	}
	// Each leading column costs its width, a space either side, and the
	// border that follows it; the border opening the row costs one more.
	prefix := 1
	for _, width := range widths[:len(widths)-1] {
		prefix += width + 3
	}
	if left := terminal - prefix - 1; left >= 20 {
		return left
	}
	return 0
}

// module renders, and under --apply performs, the resolution of one module.
// It returns the reason the run has to stop, which is a working tree holding
// changes resolve did not make.
func (r *resolveRun) module(plan resolvePlan) (string, error) {
	// A repository without a go.mod has no requirements to move and nothing
	// for the go tool to tidy, so it goes straight to its git state.
	committed := true
	if plan.GoModule {
		// The go directive is raised first, so the update that follows
		// resolves requirements against the version the module ends up on.
		if plan.GoTo != "" {
			if err := r.setGo(plan); err != nil {
				return "", err
			}
		}

		// Every go get is tidied straight after it, so the requirement it
		// replaced is out of go.mod and its checksums are out of go.sum
		// before the next one is asked for.
		if err := r.update(plan.Path, nil, "go", "get", "-u", "./..."); err != nil {
			return "", err
		}
		for _, pin := range plan.Pins {
			// A tag pushed moments ago may not have reached the module proxy
			// yet, so the source is tried before the pin is given up on.
			version := pin.version
			if actual, ok := r.actual[pin.path]; ok {
				version = actual
			}
			if err := r.update(plan.Path, []string{"GOPROXY=direct"}, "go", "get", pin.path+"@"+version); err != nil {
				return "", err
			}
		}

		var err error
		if committed, err = r.commit(plan); err != nil {
			return "", err
		}
	}

	// Nothing of this module changed and nothing of its dependencies did
	// either, so there is no release to make.
	if plan.Conditional && r.apply && !committed {
		r.add(components.ColorGreen, "Already up to date.")
		r.record(plan, plan.Latest)
		return "", nil
	}

	switch {
	case plan.GoTo != "":
		r.add(components.ColorSeparator, "go: %s", goVersionChange(plan.GoFrom, plan.GoTo))
	case plan.GoSince != "" && goSeriesChanged(plan.GoSince, plan.GoFrom):
		r.add(components.ColorSeparator, "go: %s since %s", goVersionChange(plan.GoSince, plan.GoFrom), plan.Latest)
	}

	r.add(components.ColorSeparator, "%s", plan.API.Summary())
	if r.verbose {
		for _, line := range plan.API.Symbols() {
			r.add(components.ColorSeparator, "%s", line)
		}
	}

	// The dirty check is what the working tree says now under --apply, and
	// the prediction made while planning otherwise. Either way it is read
	// after the go.mod commit, which is why that file is left out of it.
	dirty := plan.Dirty
	if r.apply {
		dirty = dirtyFiles(plan.Path)
	}
	if len(dirty) > 0 {
		for _, file := range dirty {
			r.add(components.ColorAmber, "%s", file)
		}
		return "working tree is dirty", nil
	}

	if plan.Release == "" {
		r.record(plan, plan.Latest)
		return "", nil
	}
	if plan.Conditional && !r.apply {
		r.add(components.ColorSeparator, "released only if go.mod, go.sum change")
	}

	tags, _, err := moduleTags(plan.Path)
	if err != nil {
		return "", err
	}
	steps, err := releaseSteps(tags, plan.Release, plan.TagPrefix)
	if err != nil {
		return "", err
	}
	for _, step := range steps {
		if err := r.run(plan.Path, step...); err != nil {
			return "", err
		}
	}
	r.record(plan, plan.Next)
	return "", nil
}

// record notes the version a module ended at, so a module depending on it asks
// for the one it actually got rather than the one the plan predicted. A
// conditional module that turned out to need no release is the case this is
// for.
func (r *resolveRun) record(plan resolvePlan, version string) {
	if r.actual == nil {
		r.actual = make(map[string]string)
	}
	r.actual[plan.Module] = version
}

// commit records the go.mod and go.sum this run rewrote. They are staged
// first, since a go.sum created by the update is not yet tracked and the
// pathspec would not match it, and then committed by pathspec, which leaves
// every other change in the working tree out of the commit and there for the
// dirty check to find.
// It reports whether a commit was made, which is what tells a module released
// only for a dependency update from one with nothing to release.
func (r *resolveRun) commit(plan resolvePlan) (bool, error) {
	paths := modPaths(plan.Path)
	if len(paths) == 0 {
		return false, nil
	}

	name := filepath.Base(plan.Path)
	if name == "." || name == string(filepath.Separator) {
		name = components.ShortName(plan.Module)
	}
	message := name + ": update " + strings.Join(paths, ", ")

	// Nothing to record is not a failure: go get may have found every
	// requirement already at the version it wanted.
	if r.apply {
		out, err := r.exec(plan.Path, nil, append([]string{"git", "status", "--porcelain", "--"}, paths...)...)
		if err == nil && strings.TrimSpace(out) == "" {
			return false, nil
		}
	}

	if err := r.run(plan.Path, append([]string{"git", "add", "--"}, paths...)...); err != nil {
		return false, err
	}
	if err := r.run(plan.Path, append([]string{"git", "commit", "-m", message, "--"}, paths...)...); err != nil {
		return false, err
	}
	return true, nil
}

// modPaths returns the go.mod and go.sum of a module that exist, in the order
// they are committed.
func modPaths(dir string) []string {
	var paths []string
	for _, name := range []string{"go.mod", "go.sum"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			paths = append(paths, name)
		}
	}
	return paths
}

// run performs one command in the module directory, or renders it when --apply
// was not given.
func (r *resolveRun) run(dir string, args ...string) error {
	return r.retry(dir, nil, args...)
}

// retry is run with a second attempt: when the first fails and env is set, the
// command is run again with it before the failure is reported.
func (r *resolveRun) retry(dir string, env []string, args ...string) error {
	line := shellJoin(args)
	if !r.apply {
		r.add("", "%s", line)
		return nil
	}

	out, err := r.exec(dir, nil, args...)
	if err != nil && len(env) > 0 {
		out, err = r.exec(dir, env, args...)
	}
	if err != nil {
		r.add(components.ColorRed, "%s", line)
		r.output(out)
		return fmt.Errorf("%s: %w", line, err)
	}

	r.add("", "%s%s", line, r.check())
	if r.verbose {
		r.output(out)
	}
	return nil
}

// check is the mark a command that ran carries, the one runCommand uses for
// the same purpose.
func (r *resolveRun) check() string {
	if !r.styled {
		return ""
	}
	return " " + components.ColorGreen + "✓" + components.ColorReset
}

// output writes what a command printed, indented under it.
func (r *resolveRun) output(out string) {
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			r.add(components.ColorSeparator, "  %s", line)
		}
	}
}

// shellJoin renders a command the way it would be typed. Commands are run
// through no shell, so this is only ever read, but an argument holding a space
// still has to read as one argument.
func shellJoin(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "" || strings.ContainsAny(arg, " \t\"'$&|;<>()*?[]{}#~!") {
			arg = "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
		}
		quoted = append(quoted, arg)
	}
	return strings.Join(quoted, " ")
}

// exec runs a command in dir with extra environment, returning its combined
// output.
func (r *resolveRun) exec(dir string, env []string, args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}
