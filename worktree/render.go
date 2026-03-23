package main

import (
	"fmt"
	"path"
	"strings"
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

// cell holds both a plain string (for width calculation) and a colored string (for display).
type cell struct {
	plain   string
	colored string
}

func plainCell(s string) cell {
	return cell{plain: s, colored: s}
}

func coloredCell(plain, colored string) cell {
	return cell{plain: plain, colored: colored}
}

func emptyCell() cell {
	return cell{}
}

func shortName(modPath string) string {
	return path.Base(modPath)
}

func formatModuleCell(name string) cell {
	return coloredCell(name, colorGreenLt+name+colorReset)
}

func formatLatestCell(m moduleInfo) cell {
	if m.Latest == "" {
		return plainCell("")
	}
	return coloredCell(m.Latest, colorWhite+m.Latest+colorReset)
}

func formatGitCell(m moduleInfo, st *gitStatus, verbose bool) cell {
	var plainParts, coloredParts []string

	if verbose && m.Ahead > 0 {
		s := "Unreleased changes:"
		plainParts = append(plainParts, s)
		coloredParts = append(coloredParts, colorAmber+s+colorReset)
	}
	if st != nil {
		if st.Unpushed > 0 {
			s := fmt.Sprintf("%d unpushed", st.Unpushed)
			plainParts = append(plainParts, s)
			coloredParts = append(coloredParts, colorRed+s+colorReset)
		}
		// Show "Local changes:" if there are diff lines but no other status
		if len(plainParts) == 0 && len(st.DiffLines) > 0 {
			s := "Local changes:"
			plainParts = append(plainParts, s)
			coloredParts = append(coloredParts, colorAmber+s+colorReset)
		}
	}

	if len(plainParts) == 0 {
		return plainCell("")
	}
	sep := colorBorder + ", " + colorReset
	return coloredCell(strings.Join(plainParts, ", "), strings.Join(coloredParts, sep))
}

func formatCommitMsgCell(line string) cell {
	// Parse --oneline format: "<hash> <message>" with "- " prefix
	hash, msg, found := strings.Cut(line, " ")
	plain := "- " + line
	if !found {
		return coloredCell(plain, "- "+colorWhite+line+colorReset)
	}
	colored := "- " + colorTeal + hash + colorReset + " " + colorWhite + msg + colorReset
	return coloredCell(plain, colored)
}

func formatDiffStatCell(line string) cell {
	// Format: "filename +X/-Y" with "- " prefix, colorized +/- numbers
	plain := "- " + line
	// Find last space before delta
	lastSpace := strings.LastIndex(line, " ")
	if lastSpace == -1 {
		return coloredCell(plain, "- "+line)
	}
	file, delta := line[:lastSpace], line[lastSpace+1:]
	// Parse +X/-Y
	deltaParts := strings.Split(delta, "/")
	if len(deltaParts) != 2 {
		return coloredCell(plain, "- "+line)
	}
	ins, del := deltaParts[0], deltaParts[1]
	colored := "- " + file + " " + colorGreen + ins + colorReset + "/" + colorRed + del + colorReset
	return coloredCell(plain, colored)
}

func formatDepListCell(paths []string) cell {
	if len(paths) == 0 {
		return plainCell("")
	}
	var plain, colored []string
	for _, p := range paths {
		name := shortName(p)
		plain = append(plain, name)
		colored = append(colored, colorWhite+name+colorReset)
	}
	return coloredCell(
		strings.Join(plain, ", "),
		strings.Join(colored, colorBorder+", "+colorReset),
	)
}

func formatCountCell(count int) cell {
	if count == 0 {
		return plainCell("")
	}
	s := fmt.Sprintf("%d", count)
	return coloredCell(s, colorWhite+s+colorReset)
}

// formatUsedByCell colors each dependent green if it uses the latest version of modPath, orange otherwise.
func formatUsedByCell(modPath string, usedByPaths []string, versionRefs map[string]map[string]string, latestTags map[string]string) cell {
	if len(usedByPaths) == 0 {
		return plainCell("")
	}
	latest := latestTags[modPath]
	var plain, colored []string
	for _, dep := range usedByPaths {
		name := shortName(dep)
		plain = append(plain, name)
		c := colorGreen
		if latest != "" {
			if refs, ok := versionRefs[dep]; ok {
				if ver, ok := refs[modPath]; ok && ver != latest {
					c = colorYellow
				}
			}
		}
		colored = append(colored, c+name+colorReset)
	}
	return coloredCell(
		strings.Join(plain, ", "),
		strings.Join(colored, colorBorder+", "+colorReset),
	)
}

// formatUsedByCountCell shows count with color: green if all up-to-date, orange if any outdated.
func formatUsedByCountCell(modPath string, usedByPaths []string, versionRefs map[string]map[string]string, latestTags map[string]string) cell {
	if len(usedByPaths) == 0 {
		return plainCell("")
	}
	latest := latestTags[modPath]
	hasOutdated := false
	for _, dep := range usedByPaths {
		if latest != "" {
			if refs, ok := versionRefs[dep]; ok {
				if ver, ok := refs[modPath]; ok && ver != latest {
					hasOutdated = true
					break
				}
			}
		}
	}
	s := fmt.Sprintf("%d", len(usedByPaths))
	c := colorGreen
	if hasOutdated {
		c = colorYellow
	}
	return coloredCell(s, c+s+colorReset)
}

// tableRow represents one logical row which may span multiple display lines.
type tableRow struct {
	lines [][]cell // lines[lineIdx][colIdx]
}

func formatPathCell(path string) cell {
	return coloredCell(path, colorAmber+path+colorReset)
}

func formatTitleCell(m moduleInfo) (first cell, second cell) {
	// Trim short title prefix (e.g., "ShortName - Description" -> "Description")
	title := m.Description
	if _, after, ok := strings.Cut(title, " - "); ok {
		title = after
	}

	// First line: description (path) - description in bright white, path in amber with white braces
	if title == "" {
		first = formatPathCell(m.Path)
	} else {
		descPath := title + " (" + m.Path + ")"
		coloredDescPath := colorWhite + title + colorReset + " " + colorWhite + "(" + colorAmber + m.Path + colorWhite + ")" + colorReset
		first = coloredCell(descPath, coloredDescPath)
	}

	// Second line: import path (teal, same as git branch)
	second = coloredCell(m.Name, colorTeal+m.Name+colorReset)
	return
}

func formatGitBranchCell(branch string, ahead int) cell {
	if branch == "" {
		return plainCell("")
	}
	c := colorTeal
	if branch != "main" {
		c = colorAmber
	}

	// Show commits ahead of release tag
	var suffix, coloredSuffix string
	if ahead > 0 {
		suffix = fmt.Sprintf(" (+%d ahead)", ahead)
		coloredSuffix = fmt.Sprintf(" %s(%s+%d ahead%s)%s", colorWhite, colorAmber, ahead, colorWhite, colorReset)
	}

	plain := branch + suffix
	colored := c + branch + colorReset + coloredSuffix
	return coloredCell(plain, colored)
}

func renderTables(modules []moduleInfo, versionRefs map[string]map[string]string, latestTags map[string]string, gitStatuses map[string]*gitStatus, verbose bool) {
	var headers []string
	var gitStateCol int

	var moduleCol int
	if verbose {
		headers = []string{"Module", "Latest", "Git Branch", "Git State", "Used By", "Uses"}
		moduleCol = 0
		gitStateCol = 3
	} else {
		headers = []string{"Module", "Latest", "Git Branch", "Git State", "Used", "Uses"}
		moduleCol = 0
		gitStateCol = 3
	}
	numCols := len(headers)

	var rows []tableRow
	for _, m := range modules {
		gitCell := formatGitCell(m, gitStatuses[m.Name], verbose)

		var tr tableRow
		var titleCell, moduleNameCell cell
		if verbose {
			titleCell, moduleNameCell = formatTitleCell(m)
		}

		if verbose {
			first := []cell{
				titleCell,
				formatLatestCell(m),
				formatGitBranchCell(m.GitBranch, m.Ahead),
				gitCell,
				formatUsedByCell(m.Name, m.UsedBy, versionRefs, latestTags),
				formatDepListCell(m.Uses),
			}
			tr = tableRow{lines: [][]cell{first}}
		} else {
			first := []cell{
				formatPathCell(m.Path),
				formatLatestCell(m),
				formatGitBranchCell(m.GitBranch, m.Ahead),
				gitCell,
				formatUsedByCountCell(m.Name, m.UsedBy, versionRefs, latestTags),
				formatCountCell(len(m.Uses)),
			}
			tr = tableRow{lines: [][]cell{first}}
		}

		// In verbose mode, build combined extra lines for Module and Git State columns
		if verbose {
			// Collect all git state lines
			var gitLines []cell
			for _, msg := range m.GitMsgs {
				gitLines = append(gitLines, formatCommitMsgCell(msg))
			}
			if st := gitStatuses[m.Name]; st != nil && len(st.DiffLines) > 0 {
				if len(m.GitMsgs) > 0 {
					gitLines = append(gitLines, coloredCell("Local changes:", colorAmber+"Local changes:"+colorReset))
				}
				for _, line := range st.DiffLines {
					gitLines = append(gitLines, formatDiffStatCell(line))
				}
			}

			// Collect module info lines (path - importpath on second line)
			var moduleLines []cell
			if moduleNameCell.plain != "" {
				moduleLines = append(moduleLines, moduleNameCell)
			}

			// Show git lines with module info
			for i := 0; i < len(gitLines); i++ {
				extra := make([]cell, numCols)
				for j := range extra {
					extra[j] = emptyCell()
				}
				if i < len(moduleLines) {
					extra[moduleCol] = moduleLines[i]
				}
				extra[gitStateCol] = gitLines[i]
				tr.lines = append(tr.lines, extra)
			}
			// Add remaining module lines (after git content is exhausted)
			for i := len(gitLines); i < len(moduleLines); i++ {
				extra := make([]cell, numCols)
				for j := range extra {
					extra[j] = emptyCell()
				}
				extra[moduleCol] = moduleLines[i]
				tr.lines = append(tr.lines, extra)
			}
		} else {
			// Add diff stat lines for non-verbose
			if st := gitStatuses[m.Name]; st != nil && len(st.DiffLines) > 0 {
				for _, line := range st.DiffLines {
					extra := make([]cell, numCols)
					for i := range extra {
						extra[i] = emptyCell()
					}
					extra[gitStateCol] = formatDiffStatCell(line)
					tr.lines = append(tr.lines, extra)
				}
			}
		}

		rows = append(rows, tr)
	}

	// Compute column widths across all lines of all rows
	widths := make([]int, numCols)
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, tr := range rows {
		for _, line := range tr.lines {
			for i, c := range line {
				if len(c.plain) > widths[i] {
					widths[i] = len(c.plain)
				}
			}
		}
	}

	// Top border
	printBorder(boxTopLeft, boxTeeDown, boxTopRight, colorSeparator, widths)

	// Header row
	printHeaderRow(headers, widths)

	// Header separator
	printBorder(boxTeeRight, boxCross, boxTeeLeft, colorSeparator, widths)

	// Data rows
	for i, tr := range rows {
		for _, line := range tr.lines {
			printCellRow(line, widths)
		}
		// Print separator between rows (not after last row) in verbose mode
		if verbose && i < len(rows)-1 {
			printBorder(boxTeeRight, boxCross, boxTeeLeft, colorSeparator, widths)
		}
	}

	// Bottom border
	printBorder(boxBottomLeft, boxTeeUp, boxBottomRight, colorSeparator, widths)

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
		fmt.Printf("%srun with %s-u%s %sto update %d outdated dependencies in workspace%s\n", colorBorder, colorYellow, colorReset, colorBorder, outdated, colorReset)
	}
}

func printBorder(left, mid, right, color string, widths []int) {
	var segs []string
	for _, w := range widths {
		segs = append(segs, strings.Repeat(boxHorizontal, w+2))
	}
	fmt.Println(color + left + strings.Join(segs, mid) + right + colorReset)
}

func printHeaderRow(headers []string, widths []int) {
	var cells []string
	for i, h := range headers {
		cells = append(cells, fmt.Sprintf(" %s%-*s%s ", colorHeader, widths[i], h, colorReset))
	}
	fmt.Println(colorSeparator + boxVertical + colorReset +
		strings.Join(cells, colorSeparator+boxVertical+colorReset) +
		colorSeparator + boxVertical + colorReset)
}

func printCellRow(row []cell, widths []int) {
	var cells []string
	for i, c := range row {
		pad := widths[i] - len(c.plain)
		cells = append(cells, " "+c.colored+strings.Repeat(" ", pad)+" ")
	}
	fmt.Println(colorSeparator + boxVertical + colorReset +
		strings.Join(cells, colorSeparator+boxVertical+colorReset) +
		colorSeparator + boxVertical + colorReset)
}
