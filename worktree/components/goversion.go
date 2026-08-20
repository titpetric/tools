package components

// GoVersion formats the go directive of a go.mod. An outdated version, one
// below the highest the workspace declares, is coloured amber.
func GoVersion(version string, outdated bool) Cell {
	if version == "" {
		return nil
	}
	color := ColorTeal
	if outdated {
		color = ColorAmber
	}
	return Cell{color + version + ColorReset}
}
