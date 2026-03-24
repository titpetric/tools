package components

import (
	"fmt"
	"strings"
)

// Git holds git state for rendering cells.
type Git struct {
	BranchName string
	Ahead      int
	Unpushed   int
	Msgs       []string
	DiffLines  []string
}

// Branch formats the git branch with optional commits-ahead indicator.
func (g Git) Branch() Cell {
	if g.BranchName == "" {
		return nil
	}
	c := ColorTeal
	if g.BranchName != "main" {
		c = ColorAmber
	}
	line := c + g.BranchName + ColorReset
	if g.Ahead > 0 {
		line += fmt.Sprintf(" %s(%s+%d ahead%s)%s", ColorWhite, ColorAmber, g.Ahead, ColorWhite, ColorReset)
	}
	return Cell{line}
}

// State builds a compact git state cell (summary line + diff stats).
func (g Git) State() Cell {
	var lines Cell

	var parts []string
	if g.Unpushed > 0 {
		parts = append(parts, ColorRed+fmt.Sprintf("%d unpushed", g.Unpushed)+ColorReset)
	}
	if len(parts) == 0 && len(g.DiffLines) > 0 {
		parts = append(parts, ColorAmber+"Local changes:"+ColorReset)
	}
	if len(parts) > 0 {
		lines = append(lines, strings.Join(parts, ColorBorder+", "+ColorReset))
	}

	for _, line := range g.DiffLines {
		lines = append(lines, formatDiffStatLine(line))
	}

	return lines
}

// StateVerbose builds a verbose git state cell with commit messages and diff stats.
func (g Git) StateVerbose() Cell {
	var lines Cell

	var summaryParts []string
	if g.Ahead > 0 {
		summaryParts = append(summaryParts, ColorAmber+"Unreleased changes:"+ColorReset)
	}
	if g.Unpushed > 0 {
		summaryParts = append(summaryParts, ColorRed+fmt.Sprintf("%d unpushed", g.Unpushed)+ColorReset)
	}
	if len(summaryParts) == 0 && len(g.DiffLines) > 0 {
		summaryParts = append(summaryParts, ColorAmber+"Local changes:"+ColorReset)
	}
	if len(summaryParts) > 0 {
		lines = append(lines, strings.Join(summaryParts, ColorBorder+", "+ColorReset))
	}

	for _, msg := range g.Msgs {
		lines = append(lines, formatCommitMsgLine(msg))
	}

	if len(g.DiffLines) > 0 {
		if len(g.Msgs) > 0 {
			lines = append(lines, Separator)
			lines = append(lines, ColorAmber+"Local changes:"+ColorReset)
		}
		for _, line := range g.DiffLines {
			lines = append(lines, formatDiffStatLine(line))
		}
	}

	return lines
}

func formatDiffStatLine(line string) string {
	lastSpace := strings.LastIndex(line, " ")
	if lastSpace == -1 {
		return "- " + line
	}
	file, delta := line[:lastSpace], line[lastSpace+1:]
	deltaParts := strings.Split(delta, "/")
	if len(deltaParts) != 2 {
		return "- " + line
	}
	ins, del := deltaParts[0], deltaParts[1]
	return "- " + file + " " + ColorGreen + ins + ColorReset + "/" + ColorRed + del + ColorReset
}

func formatCommitMsgLine(line string) string {
	hash, msg, found := strings.Cut(line, " ")
	if !found {
		return "- " + ColorWhite + line + ColorReset
	}
	return "- " + ColorTeal + hash + ColorReset + " " + ColorWhite + msg + ColorReset
}
