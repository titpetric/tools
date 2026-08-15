package main

import (
	"fmt"
	"io"
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

func renderTables(w io.Writer, modules []moduleInfo, opts *Options, styled bool) {
	headers := []string{"Module", "Latest", "Git Branch", "Git State", "Usage"}
	numCols := len(headers)

	// Check if all modules would be skipped; if so, show them all (only when not verbose)
	if !opts.All && !opts.Verbose {
		allSkipped := true
		for _, m := range modules {
			g := m.GitState
			if !g.State().Empty() || m.Outdated > 0 {
				allSkipped = false
				break
			}
		}
		if allSkipped {
			opts.All = true
		}
	}

	var rows []components.Rows
	for _, m := range modules {
		cells := make(components.Rows, numCols)

		g := m.GitState

		if opts.Verbose {
			cells[0] = components.ModuleVerbose(m.Description, m.Path, m.Name)
			cells[1] = components.Latest(m.Latest)
			cells[2] = g.Branch()
			cells[3] = g.StateVerbose()
			cells[4] = m.Usage.Verbose()
		} else {
			cells[0] = components.Module(m.Path)
			cells[1] = components.Latest(m.Latest)
			cells[2] = g.Branch()
			cells[3] = g.State()
			cells[4] = m.Usage.Compact()
		}

		// Skip modules where git state and usage cells are both empty
		if !opts.All && cells[3].Empty() && m.Outdated == 0 {
			opts.Skipped++
			continue
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
	if styled {
		writeBorder(w, boxTopLeft, boxTeeDown, boxTopRight, widths)
		writeHeaderRow(w, headers, widths)
		writeBorder(w, boxTeeRight, boxCross, boxTeeLeft, widths)
		for i, row := range rows {
			writeTableRow(w, row, widths)
			if opts.Verbose && i < len(rows)-1 {
				writeBorder(w, boxTeeRight, boxCross, boxTeeLeft, widths)
			}
		}
		writeBorder(w, boxBottomLeft, boxTeeUp, boxBottomRight, widths)
	} else {
		writeMarkdownTable(w, headers, rows)
	}

	headerColor, borderColor, yellow, reset := "", "", "", ""
	if styled {
		headerColor, borderColor = components.ColorHeader, components.ColorBorder
		yellow, reset = components.ColorYellow, components.ColorReset
	}

	// Count outdated dependencies
	outdated := 0
	for _, m := range modules {
		outdated += m.Outdated
	}
	if outdated > 0 {
		fmt.Fprintf(w, "%srun with %s-u%s %sto update %d outdated dependencies in workspace%s\n",
			borderColor, yellow, reset, borderColor, outdated, reset)
	}

	// Print skipped summary
	if opts.Skipped > 0 {
		fmt.Fprintf(w, "%sSkipped %d modules, use --all to show%s\n",
			headerColor, opts.Skipped, reset)
	}
}

func buildUsage(refs versionRefs, tags latestTags, m moduleInfo) (components.Usage, int) {
	var u components.Usage
	outdated := 0
	for _, dep := range m.UsedBy {
		d := components.Dependent{Name: components.ShortName(dep)}
		if dependencyOutdated(refs, tags, dep, m.Name) {
			d.Outdated = true
			outdated++
		}
		u.UsedBy = append(u.UsedBy, d)
	}
	for _, dep := range m.Uses {
		u.Uses = append(u.Uses, components.ShortName(dep))
	}
	return u, outdated
}

func dependencyOutdated(refs versionRefs, tags latestTags, dependent, dependency string) bool {
	latest := tags[dependency]
	if latest == "" {
		return false
	}
	version, ok := refs[dependent][dependency]
	return ok && version != latest
}

func writeBorder(w io.Writer, left, mid, right string, widths []int) {
	var segs []string
	for _, width := range widths {
		segs = append(segs, strings.Repeat(boxHorizontal, width+2))
	}
	fmt.Fprintln(w, components.ColorSeparator+left+strings.Join(segs, mid)+right+components.ColorReset)
}

func writeHeaderRow(w io.Writer, headers []string, widths []int) {
	var cells []string
	for i, h := range headers {
		pad := widths[i] - ansi.StringWidth(h)
		cells = append(cells, fmt.Sprintf(" %s%s%s%s ", components.ColorHeader, h, strings.Repeat(" ", pad), components.ColorReset))
	}
	fmt.Fprintln(w, components.ColorSeparator+boxVertical+components.ColorReset+
		strings.Join(cells, components.ColorSeparator+boxVertical+components.ColorReset)+
		components.ColorSeparator+boxVertical+components.ColorReset)
}

func writeTableRow(w io.Writer, row components.Rows, widths []int) {
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
		fmt.Fprintln(w, components.ColorSeparator+boxVertical+components.ColorReset+
			strings.Join(cells, components.ColorSeparator+boxVertical+components.ColorReset)+
			components.ColorSeparator+boxVertical+components.ColorReset)
	}
}
