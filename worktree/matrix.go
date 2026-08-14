package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/titpetric/tools/worktree/components"
)

const dependencyMark = "▲"

func renderDependencyMatrix(w io.Writer, modules []moduleInfo, refs versionRefs, tags latestTags) {
	available := make(map[string]struct{}, len(modules))
	for _, module := range modules {
		available[module.Name] = struct{}{}
	}

	used := make(map[string]struct{})
	var rows []moduleInfo
	for _, module := range modules {
		hasDependency := false
		for _, dependency := range module.Uses {
			if _, ok := available[dependency]; ok {
				used[dependency] = struct{}{}
				hasDependency = true
			}
		}
		if hasDependency {
			rows = append(rows, module)
		}
	}

	var columns []moduleInfo
	for _, module := range modules {
		if _, ok := used[module.Name]; ok {
			columns = append(columns, module)
		}
	}

	labels := make([]string, len(columns))
	widths := make([]int, len(columns)+1)
	widths[0] = ansi.StringWidth("Project")
	for i, module := range columns {
		labels[i] = components.ShortName(module.Name)
		widths[i+1] = max(ansi.StringWidth(labels[i]), 5)
	}
	for _, module := range rows {
		widths[0] = max(widths[0], ansi.StringWidth(components.ShortName(module.Name)))
	}

	writeBorder(w, boxTopLeft, boxTeeDown, boxTopRight, widths)
	headers := append([]string{"Project"}, labels...)
	for i, header := range headers {
		headers[i] = components.ColorHeader + header + components.ColorReset
	}
	writeMatrixRow(w, headers, widths, false)
	writeBorder(w, boxTeeRight, boxCross, boxTeeLeft, widths)

	for _, module := range rows {
		dependencies := make(map[string]struct{}, len(module.Uses))
		for _, dependency := range module.Uses {
			dependencies[dependency] = struct{}{}
		}

		row := make([]string, len(widths))
		row[0] = components.ColorAmber + components.ShortName(module.Name) + components.ColorReset
		for i, candidate := range columns {
			if _, ok := dependencies[candidate.Name]; ok {
				color := components.ColorGreen
				if dependencyOutdated(refs, tags, module.Name, candidate.Name) {
					color = components.ColorYellow
				}
				row[i+1] = color + dependencyMark + components.ColorReset
				if gitTreeDirty(candidate) {
					row[i+1] += components.ColorDarkOrange + "*" + components.ColorReset
				}
			}
		}
		writeMatrixRow(w, row, widths, true)
	}
	writeBorder(w, boxBottomLeft, boxTeeUp, boxBottomRight, widths)
}

func writeMatrixRow(w io.Writer, cells []string, widths []int, centerData bool) {
	separator := components.ColorSeparator + boxVertical + components.ColorReset
	fmt.Fprint(w, separator)
	for i, cell := range cells {
		left, right := 0, widths[i]-ansi.StringWidth(cell)
		if centerData && i > 0 {
			left = right / 2
			right -= left
		}
		fmt.Fprintf(w, " %s%s%s %s", strings.Repeat(" ", left), cell, strings.Repeat(" ", right), separator)
	}
	fmt.Fprintln(w)
}

func gitTreeDirty(module moduleInfo) bool {
	return module.GitState != nil && (len(module.GitState.DiffLines) > 0 || len(module.GitState.UntrackedFiles) > 0)
}
