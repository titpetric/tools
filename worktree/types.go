package main

import "github.com/titpetric/tools/worktree/components"

type gitStatus struct {
	Unpushed  int
	Modified  int
	DiffLines []string // git diff --stat output lines
}

type moduleInfo struct {
	Name        string
	Path        string
	Description string
	Latest      string
	GitState    *components.Git
	Usage       components.Usage
	Outdated    int
	Uses        []string
	UsedBy      []string
}

type requireInfo struct {
	path    string
	version string
}

// versionRefs maps module path → dependency path → version.
type versionRefs map[string]map[string]string

// latestTags maps module path → latest tag.
type latestTags map[string]string
