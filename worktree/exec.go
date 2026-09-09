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

// execModules runs a shell command in every Go module, one bash -c invocation
// per module, streaming a status row as each finishes. Unlike -u, the command's
// output is always part of the row, not only with -v: the command is the
// caller's, so its output is the result. It returns the number of modules the
// command failed in.
func execModules(w io.Writer, modPaths map[string]string, command string, verbose, styled bool) int {
	mods := make([]string, 0, len(modPaths))
	for modPath := range modPaths {
		mods = append(mods, modPath)
	}
	sort.Strings(mods)

	headers := []string{"Path", "Module", "Exec status"}
	widths := headerWidths(headers)
	for _, modPath := range mods {
		widths[0] = max(widths[0], ansi.StringWidth(relPath(modPaths[modPath])))
		widths[1] = max(widths[1], ansi.StringWidth(components.ShortPath(modPath)))
	}

	table := newStreamTable(w, headers, widths, styled)
	defer table.close()

	failed := 0
	for _, modPath := range mods {
		dir := modPaths[modPath]
		table.start(relPath(dir), components.ShortPath(modPath))

		s := &status{styled: styled}
		var out bytes.Buffer
		cmd := exec.Command("bash", "-c", command)
		cmd.Dir = dir
		err := runCommand(cmd, verbose, &out, &out)
		if text := strings.TrimSpace(out.String()); text != "" {
			s.log = append(s.log, strings.Split(text, "\n")...)
		}
		switch {
		case err != nil:
			failed++
			s.failed = true
			s.log = append(s.log, colorLines(fmt.Sprintf("bash -c %s: %v", command, err), components.ColorRed, styled))
		case len(s.log) == 0:
			s.add(components.ColorGreen, "Done.")
		}
		table.finish(s.String())
	}
	return failed
}
