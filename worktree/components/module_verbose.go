package components

import "strings"

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
