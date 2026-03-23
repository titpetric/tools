package components

import (
	"fmt"
	"strings"
)

// ModuleVerbose returns a verbose module cell with description, path, and import path.
func ModuleVerbose(description, dirPath, moduleName string) Cell {
	title := description
	if _, after, ok := strings.Cut(title, " - "); ok {
		title = after
	}

	var lines Cell

	if title != "" {
		lines = append(lines, ColorWhite+title+ColorReset)
	}

	lines = append(lines, ColorTeal+moduleName+ColorReset+" "+ColorWhite+"("+ColorAmber+dirPath+ColorWhite+")"+ColorReset)
	return lines
}

// GitStateVerbose builds a verbose git state cell with commit messages and diff stats.
func GitStateVerbose(ahead, unpushed int, gitMsgs, diffLines []string) Cell {
	var lines Cell

	var summaryParts []string
	if ahead > 0 {
		summaryParts = append(summaryParts, ColorAmber+"Unreleased changes:"+ColorReset)
	}
	if unpushed > 0 {
		summaryParts = append(summaryParts, ColorRed+fmt.Sprintf("%d unpushed", unpushed)+ColorReset)
	}
	if len(summaryParts) == 0 && len(diffLines) > 0 {
		summaryParts = append(summaryParts, ColorAmber+"Local changes:"+ColorReset)
	}
	if len(summaryParts) > 0 {
		lines = append(lines, strings.Join(summaryParts, ColorBorder+", "+ColorReset))
	}

	for _, msg := range gitMsgs {
		lines = append(lines, formatCommitMsgLine(msg))
	}

	if len(diffLines) > 0 {
		if len(gitMsgs) > 0 {
			lines = append(lines, Separator)
			lines = append(lines, ColorAmber+"Local changes:"+ColorReset)
		}
		for _, line := range diffLines {
			lines = append(lines, formatDiffStatLine(line))
		}
	}

	return lines
}

func formatCommitMsgLine(line string) string {
	hash, msg, found := strings.Cut(line, " ")
	if !found {
		return "- " + ColorWhite + line + ColorReset
	}
	return "- " + ColorTeal + hash + ColorReset + " " + ColorWhite + msg + ColorReset
}
