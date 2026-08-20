package main

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/titpetric/tools/worktree/components"
)

// status accumulates the lines of an update status cell: summary lines first,
// followed by the command log (verbose output and errors).
type status struct {
	styled bool
	lines  []string
	log    []string
	failed bool
}

// add appends a colored summary line.
func (s *status) add(color, format string, args ...any) {
	s.lines = append(s.lines, colorLines(fmt.Sprintf(format, args...), color, s.styled))
}

// run executes a command in dir, recording its output when verbose and always
// recording failures.
func (s *status) run(dir string, verbose bool, args ...string) error {
	var out bytes.Buffer
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	err := runCommand(cmd, verbose, &out, &out)
	if text := strings.TrimSpace(out.String()); text != "" && (verbose || err != nil) {
		s.log = append(s.log, strings.Split(text, "\n")...)
	}
	if err != nil {
		s.failed = true
		s.log = append(s.log, colorLines(fmt.Sprintf("%s: %v", strings.Join(args, " "), err), components.ColorRed, s.styled))
	}
	return err
}

// empty reports whether the status has neither summary lines nor failures.
func (s *status) empty() bool {
	return len(s.lines) == 0 && !s.failed
}

// String renders the status cell.
func (s *status) String() string {
	return strings.Join(append(append([]string{}, s.lines...), s.log...), "\n")
}

// depChange describes a single go.mod requirement change.
type depChange struct {
	path string
	from string
	to   string
}

// String formats the change as a status line.
func (c depChange) String() string {
	switch {
	case c.to == "":
		return "- " + c.path + " " + c.from
	case c.from == "":
		return "+ " + c.path + " " + c.to
	default:
		return c.path + " " + c.from + " → " + c.to
	}
}

// Color returns the status color for the change: removals are dimmed, new
// requirements are green, and version changes to an existing requirement are
// amber.
func (c depChange) Color() string {
	switch {
	case c.to == "":
		return components.ColorSeparator
	case c.from == "":
		return components.ColorGreen
	default:
		return components.ColorAmber
	}
}

// diffRequires compares two go.mod requirement sets, returning the changes
// sorted by module path.
func diffRequires(before, after []requireInfo) []depChange {
	versions := func(reqs []requireInfo) map[string]string {
		out := make(map[string]string, len(reqs))
		for _, r := range reqs {
			out[r.path] = r.version
		}
		return out
	}
	old, cur := versions(before), versions(after)

	paths := make([]string, 0, len(old)+len(cur))
	for path := range old {
		paths = append(paths, path)
	}
	for path := range cur {
		if _, ok := old[path]; !ok {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)

	var changes []depChange
	for _, path := range paths {
		if old[path] == cur[path] {
			continue
		}
		changes = append(changes, depChange{path: path, from: old[path], to: cur[path]})
	}
	return changes
}

// updateDeps updates dependencies in each Go module, printing each module's
// go.mod changes as soon as they are known. When opts.GoVersion is set, the go
// directive of each go.mod is rewritten before the dependencies are updated;
// a module already declaring that version is reported as up to date and
// skipped, unless -u asked for a dependency update as well.
func updateDeps(w io.Writer, modPaths map[string]string, tags latestTags, opts *Options, styled bool) {
	verbose := opts.Verbose

	mods := make([]string, 0, len(modPaths))
	for modPath := range modPaths {
		mods = append(mods, modPath)
	}
	sort.Strings(mods)

	headers := []string{"Path", "Module", "Update status"}
	widths := headerWidths(headers)
	for _, modPath := range mods {
		widths[0] = max(widths[0], ansi.StringWidth(relPath(modPaths[modPath])))
		widths[1] = max(widths[1], ansi.StringWidth(components.ShortPath(modPath)))
	}

	table := newStreamTable(w, headers, widths, styled)
	defer table.close()

	for _, modPath := range mods {
		dir := modPaths[modPath]
		table.start(relPath(dir), components.ShortPath(modPath))

		s := &status{styled: styled}
		if opts.GoVersion != "" {
			prev, err := setGoVersion(dir, opts.GoVersion)
			switch {
			case err != nil:
				s.failed = true
				s.add(components.ColorRed, "failed to set go version: %v", err)
			case prev != opts.GoVersion:
				s.add(components.ColorAmber, "%s", goVersionChange(prev, opts.GoVersion))
			case !opts.Update:
				// The go.mod already declares the version, so there is
				// nothing to rewrite and no reason to run the go tool.
				s.add(components.ColorGreen, "Already up to date.")
				table.finish(s.String())
				continue
			}
		}
		before, _ := readRequiresVersioned(dir)
		s.run(dir, verbose, "go", "get", "-u", "./...")

		// Update workspace dependencies to their latest tags
		reqs, err := readRequiresVersioned(dir)
		if err != nil {
			s.failed = true
			s.add(components.ColorRed, "failed to read go.mod: %v", err)
		}
		for _, r := range reqs {
			if tag, ok := tags[r.path]; ok && tag != "" && r.version != tag {
				s.run(dir, verbose, "go", "get", r.path+"@"+tag)
			}
		}

		s.run(dir, verbose, "go", "mod", "tidy")

		after, _ := readRequiresVersioned(dir)
		for _, change := range diffRequires(before, after) {
			s.add(change.Color(), "%s", change)
		}
		if s.empty() {
			s.add(components.ColorGreen, "Already up to date.")
		}

		table.finish(s.String())
	}
}
