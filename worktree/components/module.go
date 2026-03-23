package components

import (
	"fmt"
	"path"
	"strings"
)

// ShortName returns the base name of a module path.
func ShortName(modPath string) string {
	return path.Base(modPath)
}

// Module returns a compact module cell showing just the path.
func Module(dirPath string) Cell {
	return Cell{ColorAmber + dirPath + ColorReset}
}

// Latest formats the latest git tag.
func Latest(tag string) Cell {
	if tag == "" {
		return nil
	}
	return Cell{ColorWhite + tag + ColorReset}
}

// GitBranch formats the git branch with optional commits-ahead indicator.
func GitBranch(branch string, ahead int) Cell {
	if branch == "" {
		return nil
	}
	c := ColorTeal
	if branch != "main" {
		c = ColorAmber
	}
	line := c + branch + ColorReset
	if ahead > 0 {
		line += fmt.Sprintf(" %s(%s+%d ahead%s)%s", ColorWhite, ColorAmber, ahead, ColorWhite, ColorReset)
	}
	return Cell{line}
}

// GitState builds a compact git state cell (summary line + diff stats).
func GitState(unpushed int, diffLines []string) Cell {
	var lines Cell

	var parts []string
	if unpushed > 0 {
		parts = append(parts, ColorRed+fmt.Sprintf("%d unpushed", unpushed)+ColorReset)
	}
	if len(parts) == 0 && len(diffLines) > 0 {
		parts = append(parts, ColorAmber+"Local changes:"+ColorReset)
	}
	if len(parts) > 0 {
		lines = append(lines, strings.Join(parts, ColorBorder+", "+ColorReset))
	}

	for _, line := range diffLines {
		lines = append(lines, formatDiffStatLine(line))
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
