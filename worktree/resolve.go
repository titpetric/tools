package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"

	"github.com/titpetric/tools/worktree/components"
)

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
