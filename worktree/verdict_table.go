package main

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/titpetric/tools/worktree/components"
)

// shortPackage names a package the way the tables refer to it, as its import
// path below the module, written from the module root down: "/model" is the
// model package of this module and not some other one, and the package at the
// root of the module is "/".
//
// The name alone is not enough to go on. A module holding a model package
// under two directories has two of them, both named "model", and a column
// saying so twice tells a reader nothing about which is which.
//
// A package that is not below the module is named by its import path, which is
// the only name it has here.
func shortPackage(module, pkg string) string {
	if pkg == module {
		return "/"
	}
	if rel := strings.TrimPrefix(pkg, module+"/"); rel != pkg {
		return "/" + rel
	}
	return pkg
}

// fold breaks a line to a width, and leaves it alone when there is no width to
// break it to.
func fold(line string, width int) string {
	if width <= 0 {
		return line
	}
	return ansi.Wrap(line, width, "")
}

// changeColor returns the colour a category of change is written in: green for
// what a release adds, amber for what it reshapes, red for what it takes away.
func changeColor(category string) string {
	switch strings.ToLower(category) {
	case fieldAdded:
		return components.ColorGreen
	case fieldRemoved:
		return components.ColorRed
	}
	return components.ColorAmber
}

// categoryOrder ranks the categories the way the report reads them, which is
// what a release adds first and what it takes away last.
func categoryOrder(category string) int {
	switch category {
	case fieldAdded:
		return 0
	case fieldChanged:
		return 1
	}
	return 2
}

// categoryName is the change a field underwent, as a table column names it.
func categoryName(category string) string {
	switch category {
	case fieldAdded:
		return "Added"
	case fieldChanged:
		return "Changed"
	case fieldRemoved:
		return "Removed"
	}
	return category
}
