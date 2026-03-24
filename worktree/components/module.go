package components

import "path"

// ShortName returns the base name of a module path.
func ShortName(modPath string) string {
	return path.Base(modPath)
}

// Module returns a compact module cell showing just the path.
func Module(dirPath string) Cell {
	return Cell{ColorAmber + dirPath + ColorReset}
}
