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

func renderTables(refs versionRefs, modules []moduleInfo, gitStatuses map[string]*gitStatus, tags latestTags, verbose bool) {
	headers := []string{"Module", "Latest", "Git Branch", "Git State", "Usage"}
	numCols := len(headers)

	var rows []components.Rows
	for _, m := range modules {
		st := gitStatuses[m.Name]
		cells := make(components.Rows, numCols)

		g := components.Git{
			BranchName: m.GitBranch,
			Ahead:      m.Ahead,
			Msgs:       m.GitMsgs,
		}
		if st != nil {
			g.Unpushed = st.Unpushed
			g.DiffLines = st.DiffLines
		}

		u := buildUsage(refs, m, tags)

		if verbose {
			cells[0] = components.ModuleVerbose(m.Description, m.Path, m.Name)
			cells[1] = components.Latest(m.Latest)
			cells[2] = g.Branch()
			cells[3] = g.StateVerbose()
			cells[4] = u.Verbose()
		} else {
			cells[0] = components.Module(m.Path)
			cells[1] = components.Latest(m.Latest)
			cells[2] = g.Branch()
			cells[3] = g.State()
			cells[4] = u.Compact()
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
		modRefs := refs[m.Name]
		for _, dep := range m.Uses {
			latest := tags[dep]
			if latest != "" && modRefs != nil && modRefs[dep] != "" && modRefs[dep] != latest {
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

func buildUsage(refs versionRefs, m moduleInfo, tags latestTags) components.Usage {
	var u components.Usage
	latest := tags[m.Name]
	for _, dep := range m.UsedBy {
		d := components.Dependent{Name: components.ShortName(dep)}
		if latest != "" {
			if depRefs, ok := refs[dep]; ok {
				if ver, ok := depRefs[m.Name]; ok && ver != latest {
					d.Outdated = true
				}
			}
		}
		u.UsedBy = append(u.UsedBy, d)
	}
	for _, dep := range m.Uses {
		u.Uses = append(u.Uses, components.ShortName(dep))
	}
	return u
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
