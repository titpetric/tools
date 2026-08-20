package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/titpetric/tools/worktree/components"
)

const dependencyMark = "▲"

// matrixMinWidth is the narrowest a dependency column gets, five cells wide
// once the single space of padding on each side is counted.
const matrixMinWidth = 3

func renderDependencyMatrix(w io.Writer, modules []moduleInfo, refs versionRefs, tags latestTags, styled bool) {
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
		if hasDependency || gitTreeDirty(module) {
			rows = append(rows, module)
		}
	}

	var columns []moduleInfo
	for _, module := range modules {
		if _, ok := used[module.Name]; ok {
			columns = append(columns, module)
		}
	}
	if !styled {
		headers := []string{"Project"}
		for _, module := range columns {
			headers = append(headers, components.ShortName(module.Name))
		}
		var values [][]string
		for _, module := range rows {
			dependencies := make(map[string]struct{}, len(module.Uses))
			for _, dependency := range module.Uses {
				dependencies[dependency] = struct{}{}
			}
			row := make([]string, len(headers))
			row[0] = matrixProjectLabel(module)
			for i, candidate := range columns {
				if _, ok := dependencies[candidate.Name]; ok {
					row[i+1] = dependencyMark
					if dependencyOutdated(refs, tags, module.Name, candidate.Name) {
						row[i+1] += "*"
					}
				}
			}
			values = append(values, row)
		}
		writeSimpleTable(w, headers, values, false)
		ahead, localChanges, outdated := matrixSummary(modules, refs, tags)
		fmt.Fprintf(w, "%d ahead, %d with local changes, %d deps out of date.\n", ahead, localChanges, outdated)
		return
	}

	labels := make([]string, len(columns))
	widths := make([]int, len(columns)+1)
	widths[0] = ansi.StringWidth("Project")
	for i, module := range columns {
		labels[i] = components.ShortName(module.Name)
		widths[i+1] = max(ansi.StringWidth(labels[i]), matrixMinWidth)
	}
	for _, module := range rows {
		widths[0] = max(widths[0], ansi.StringWidth(matrixProjectLabel(module)))
	}

	writeBorder(w, boxTopLeft, boxTeeDown, boxTopRight, widths)
	headers := append([]string{"Project"}, labels...)
	for i, header := range headers {
		headers[i] = components.ColorHeader + header + components.ColorReset
	}
	writeMatrixRow(w, headers, widths)
	writeBorder(w, boxTeeRight, boxCross, boxTeeLeft, widths)

	for _, module := range rows {
		dependencies := make(map[string]struct{}, len(module.Uses))
		for _, dependency := range module.Uses {
			dependencies[dependency] = struct{}{}
		}

		row := make([]string, len(widths))
		row[0] = matrixProjectLabel(module)
		for i, candidate := range columns {
			if _, ok := dependencies[candidate.Name]; ok {
				color := components.ColorGreen
				mark := dependencyMark
				if dependencyOutdated(refs, tags, module.Name, candidate.Name) {
					color = components.ColorYellow
					mark += "*"
				}
				row[i+1] = color + mark + components.ColorReset
			}
		}
		writeMatrixRow(w, row, widths)
	}
	writeBorder(w, boxBottomLeft, boxTeeUp, boxBottomRight, widths)

	ahead, localChanges, outdated := matrixSummary(modules, refs, tags)
	fmt.Fprintf(w, "%s%d ahead, %d with local changes, %d deps out of date.%s\n",
		components.ColorHeader, ahead, localChanges, outdated, components.ColorReset)
}

// writeMatrixRow writes one row, every cell left aligned within its column and
// padded by a single space on each side.
func writeMatrixRow(w io.Writer, cells []string, widths []int) {
	separator := components.ColorSeparator + boxVertical + components.ColorReset
	fmt.Fprint(w, separator)
	for i, cell := range cells {
		pad := max(widths[i]-ansi.StringWidth(cell), 0)
		fmt.Fprintf(w, " %s%s %s", cell, strings.Repeat(" ", pad), separator)
	}
	fmt.Fprintln(w)
}

func gitTreeDirty(module moduleInfo) bool {
	return module.GitState != nil && (len(module.GitState.DiffLines) > 0 || len(module.GitState.UntrackedFiles) > 0)
}

func matrixProjectLabel(module moduleInfo) string {
	label := components.ColorAmber + components.ShortName(module.Name) + components.ColorReset
	if module.GitState == nil {
		return label
	}
	if module.GitState.Ahead > 0 {
		label += fmt.Sprintf(" %s(+%d)%s", components.ColorSeparator, module.GitState.Ahead, components.ColorReset)
	}
	if gitTreeDirty(module) {
		label += " " + components.ColorDarkOrange + "*" + components.ColorReset
	}
	return label
}

func matrixSummary(modules []moduleInfo, refs versionRefs, tags latestTags) (ahead, localChanges, outdated int) {
	available := make(map[string]struct{}, len(modules))
	for _, module := range modules {
		available[module.Name] = struct{}{}
	}
	for _, module := range modules {
		if module.GitState != nil {
			ahead += module.GitState.Ahead
		}
		if gitTreeDirty(module) {
			localChanges++
		}
		for _, dependency := range module.Uses {
			if _, ok := available[dependency]; ok && dependencyOutdated(refs, tags, module.Name, dependency) {
				outdated++
			}
		}
	}
	return ahead, localChanges, outdated
}
