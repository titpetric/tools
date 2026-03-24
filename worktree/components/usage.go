package components

import (
	"fmt"
	"strings"
)

// Usage holds pre-computed usage data for rendering.
type Usage struct {
	UsedBy []Dependent
	Uses   []string
}

// Compact builds a compact single-line usage cell with counts.
func (u Usage) Compact() Cell {
	var parts []string

	if len(u.UsedBy) > 0 {
		s := fmt.Sprintf("%d", len(u.UsedBy))
		c := ColorGreen
		for _, d := range u.UsedBy {
			if d.Outdated {
				c = ColorYellow
				break
			}
		}
		parts = append(parts, ColorBorder+"↑ "+c+s+ColorReset)
	}
	if len(u.Uses) > 0 {
		s := fmt.Sprintf("%d", len(u.Uses))
		parts = append(parts, ColorBorder+"↓ "+ColorWhite+s+ColorReset)
	}

	if len(parts) == 0 {
		return nil
	}
	return Cell{strings.Join(parts, " ")}
}

// Verbose builds a verbose usage cell with named dependency lists.
func (u Usage) Verbose() Cell {
	var lines Cell

	if len(u.UsedBy) > 0 {
		var parts []string
		for _, d := range u.UsedBy {
			c := ColorGreen
			if d.Outdated {
				c = ColorYellow
			}
			parts = append(parts, c+d.Name+ColorReset)
		}
		lines = append(lines, ColorBorder+"↑ "+ColorReset+strings.Join(parts, ColorBorder+", "+ColorReset))
	}

	if len(u.Uses) > 0 {
		var parts []string
		for _, name := range u.Uses {
			parts = append(parts, ColorWhite+name+ColorReset)
		}
		lines = append(lines, ColorBorder+"↓ "+ColorReset+strings.Join(parts, ColorBorder+", "+ColorReset))
	}

	return lines
}
