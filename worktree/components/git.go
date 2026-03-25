package components

import (
	"fmt"
	"strings"
)

// Issue holds a parsed GitHub issue.
type Issue struct {
	ID    string
	Title string
	Date  string
}

// Git holds git state for rendering cells.
type Git struct {
	BranchName string
	Ahead      int
	Unpushed   int
	Msgs       []string
	DiffLines  []string
	Issues     []Issue
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
		line += fmt.Sprintf(" %s(%s+%d ahead%s)%s", ColorWhite, ColorRed, g.Ahead, ColorWhite, ColorReset)
	}
	return Cell{line}
}

func (g Git) summaryLine() Cell {
	if g.Unpushed > 0 {
		return Cell{ColorRed + fmt.Sprintf("Unpushed changes: %d", g.Unpushed) + ColorReset}
	} else if len(g.DiffLines) > 0 {
		return Cell{ColorAmber + "Local changes:" + ColorReset}
	}
	return nil
}

// State builds a compact git state cell (summary line + diff stats).
func (g Git) State() Cell {
	var lines Cell

	lines = append(lines, g.summaryLine()...)

	if len(g.DiffLines) > 0 {
		if g.Ahead > 0 || g.Unpushed > 0 {
			lines = append(lines, Separator)
			lines = append(lines, ColorAmber+"Local changes:"+ColorReset)
		}
		for _, line := range g.DiffLines {
			lines = append(lines, formatDiffStatLine(line))
		}
	}

	return lines
}

// StateVerbose builds a verbose git state cell with commit messages and diff stats.
func (g Git) StateVerbose() Cell {
	var lines Cell

	lines = append(lines, g.summaryLine()...)

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

	if len(g.Issues) > 0 {
		if len(lines) > 0 {
			lines = append(lines, Separator)
		}
		lines = append(lines, ColorAmber+fmt.Sprintf("Issues: %d open", len(g.Issues))+ColorReset)

		// Compute column widths for aligned output
		idW, titleW := 0, 0
		for _, issue := range g.Issues {
			if len(issue.ID) > idW {
				idW = len(issue.ID)
			}
			if len(issue.Title) > titleW {
				titleW = len(issue.Title)
			}
		}
		for _, issue := range g.Issues {
			line := fmt.Sprintf("%s%-*s%s  %s%-*s%s  %s%s%s",
				ColorTeal, idW, issue.ID, ColorReset,
				ColorWhite, titleW, issue.Title, ColorReset,
				ColorBorder, issue.Date, ColorReset,
			)
			lines = append(lines, line)
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
