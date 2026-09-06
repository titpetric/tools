// Package broken reaches a package no configuration names, no package of the
// tree is called, no file of it writes and no requirement ends in. There is
// nothing to import, so the file is reported and left alone.
package broken

// Thing calls into a package that does not exist.
func Thing() string {
	return nope.Value()
}
