package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/titpetric/tools/worktree/components"
)

// Light rounded box-drawing characters
const (
	boxTopLeft     = "╭"
	boxTopRight    = "╮"
	boxBottomLeft  = "╰"
	boxBottomRight = "╯"
	boxHorizontal  = "─"
	boxVertical    = "│"
	boxTeeDown     = "┬"
	boxTeeUp       = "┴"
	boxTeeRight    = "├"
	boxTeeLeft     = "┤"
	boxCross       = "┼"
)

func renderTables(modules []moduleInfo, versionRefs map[string]map[string]string, latestTags map[string]string, gitStatuses map[string]*gitStatus, verbose bool) {
	headers := []string{"Module", "Latest", "Git Branch", "Git State", "Usage"}
	numCols := len(headers)

	var rows []components.Rows
	for _, m := range modules {
		st := gitStatuses[m.Name]
		cells := make(components.Rows, numCols)

		if verbose {
			cells[0] = components.ModuleVerbose(m.Description, m.Path, m.Name)
			cells[1] = components.Latest(m.Latest)
			cells[2] = components.GitBranch(m.GitBranch, m.Ahead)

			var unpushed int
			var diffLines []string
			if st != nil {
				unpushed = st.Unpushed
				diffLines = st.DiffLines
			}
			cells[3] = components.GitStateVerbose(m.Ahead, unpushed, m.GitMsgs, diffLines)
			cells[4] = components.UsageVerbose(m.Name, m.UsedBy, m.Uses, versionRefs, latestTags)
		} else {
			cells[0] = components.Module(m.Path)
			cells[1] = components.Latest(m.Latest)
			cells[2] = components.GitBranch(m.GitBranch, m.Ahead)

			var unpushed int
			var diffLines []string
			if st != nil {
				unpushed = st.Unpushed
				diffLines = st.DiffLines
			}
			cells[3] = components.GitState(unpushed, diffLines)
			cells[4] = components.UsageCompact(m.Name, m.UsedBy, m.Uses, versionRefs, latestTags)
		}

		rows = append(rows, cells)
	}

	// Compute column widths
	widths := make([]int, numCols)
	for i, h := range headers {
		widths[i] = ansi.StringWidth(h)
	}
	for _, row := range rows {
		for colIdx, cell := range row {
			if w := cell.Width(); w > widths[colIdx] {
				widths[colIdx] = w
			}
		}
	}

	// Top border
	printBorder(boxTopLeft, boxTeeDown, boxTopRight, widths)

	// Header row
	printHeaderRow(headers, widths)

	// Header separator
	printBorder(boxTeeRight, boxCross, boxTeeLeft, widths)

	// Data rows
	for i, row := range rows {
		printTableRow(row, widths)
		if verbose && i < len(rows)-1 {
			printBorder(boxTeeRight, boxCross, boxTeeLeft, widths)
		}
	}

	// Bottom border
	printBorder(boxBottomLeft, boxTeeUp, boxBottomRight, widths)

	// Count outdated dependencies
	outdated := 0
	for _, m := range modules {
		refs := versionRefs[m.Name]
		for _, dep := range m.Uses {
			latest := latestTags[dep]
			if latest != "" && refs != nil && refs[dep] != "" && refs[dep] != latest {
				outdated++
			}
		}
	}
	if outdated > 0 {
		fmt.Printf("%srun with %s-u%s %sto update %d outdated dependencies in workspace%s\n",
			components.ColorBorder, components.ColorYellow, components.ColorReset,
			components.ColorBorder, outdated, components.ColorReset)
	}
}

func printBorder(left, mid, right string, widths []int) {
	var segs []string
	for _, w := range widths {
		segs = append(segs, strings.Repeat(boxHorizontal, w+2))
	}
	fmt.Println(components.ColorSeparator + left + strings.Join(segs, mid) + right + components.ColorReset)
}

func printHeaderRow(headers []string, widths []int) {
	var cells []string
	for i, h := range headers {
		pad := widths[i] - ansi.StringWidth(h)
		cells = append(cells, fmt.Sprintf(" %s%s%s%s ", components.ColorHeader, h, strings.Repeat(" ", pad), components.ColorReset))
	}
	fmt.Println(components.ColorSeparator + boxVertical + components.ColorReset +
		strings.Join(cells, components.ColorSeparator+boxVertical+components.ColorReset) +
		components.ColorSeparator + boxVertical + components.ColorReset)
}

func printTableRow(row components.Rows, widths []int) {
	h := row.RowHeight()
	for lineIdx := 0; lineIdx < h; lineIdx++ {
		var cells []string
		for colIdx, c := range row {
			s := c.Line(lineIdx)
			if s == components.Separator {
				cells = append(cells, " "+components.ColorSeparator+strings.Repeat("─", widths[colIdx])+components.ColorReset+" ")
				continue
			}
			pad := widths[colIdx] - ansi.StringWidth(s)
			if pad < 0 {
				pad = 0
			}
			cells = append(cells, " "+s+strings.Repeat(" ", pad)+" ")
		}
		fmt.Println(components.ColorSeparator + boxVertical + components.ColorReset +
			strings.Join(cells, components.ColorSeparator+boxVertical+components.ColorReset) +
			components.ColorSeparator + boxVertical + components.ColorReset)
	}
}
