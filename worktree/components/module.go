package components

import (
	"path"
	"strings"
)

// ShortName returns the base name of a module path.
func ShortName(modPath string) string {
	return path.Base(modPath)
}

// ShortPath drops the hosting prefix of a module path, so
// "github.com/titpetric/tools" renders as "titpetric/tools".
func ShortPath(modPath string) string {
	return strings.TrimPrefix(modPath, "github.com/")
}

// Module returns a compact module cell showing just the path.
func Module(dirPath string) Cell {
	return Cell{ColorAmber + dirPath + ColorReset}
}
