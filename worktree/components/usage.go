package components

import (
	"fmt"
	"strings"
)

// UsageCompact builds a compact usage cell combining "Used by" and "Uses" counts.
func UsageCompact(modPath string, usedBy, uses []string, versionRefs map[string]map[string]string, latestTags map[string]string) Cell {
	var lines Cell

	if len(usedBy) > 0 {
		s := fmt.Sprintf("%d", len(usedBy))
		c := ColorGreen
		if hasOutdatedDependents(modPath, usedBy, versionRefs, latestTags) {
			c = ColorYellow
		}
		lines = append(lines, ColorBorder+"↑ "+c+s+ColorReset)
	}
	if len(uses) > 0 {
		s := fmt.Sprintf("%d", len(uses))
		lines = append(lines, ColorBorder+"↓ "+ColorWhite+s+ColorReset)
	}

	return lines
}

// UsageVerbose builds a verbose usage cell combining "Used by" and "Uses" lists.
func UsageVerbose(modPath string, usedBy, uses []string, versionRefs map[string]map[string]string, latestTags map[string]string) Cell {
	var lines Cell

	if len(usedBy) > 0 {
		var parts []string
		latest := latestTags[modPath]
		for _, dep := range usedBy {
			name := ShortName(dep)
			c := ColorGreen
			if latest != "" {
				if refs, ok := versionRefs[dep]; ok {
					if ver, ok := refs[modPath]; ok && ver != latest {
						c = ColorYellow
					}
				}
			}
			parts = append(parts, c+name+ColorReset)
		}
		lines = append(lines, ColorBorder+"↑ "+ColorReset+strings.Join(parts, ColorBorder+", "+ColorReset))
	}

	if len(uses) > 0 {
		var parts []string
		for _, p := range uses {
			name := ShortName(p)
			parts = append(parts, ColorWhite+name+ColorReset)
		}
		lines = append(lines, ColorBorder+"↓ "+ColorReset+strings.Join(parts, ColorBorder+", "+ColorReset))
	}

	return lines
}

func hasOutdatedDependents(modPath string, usedBy []string, versionRefs map[string]map[string]string, latestTags map[string]string) bool {
	latest := latestTags[modPath]
	if latest == "" {
		return false
	}
	for _, dep := range usedBy {
		if refs, ok := versionRefs[dep]; ok {
			if ver, ok := refs[modPath]; ok && ver != latest {
				return true
			}
		}
	}
	return false
}
