package main

import (
	"github.com/titpetric/tools/worktree/components"
)

type projectDir struct {
	Path     string
	GoModule bool
	GitRepo  bool
}

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
	GoVersion   string
	GitState    *components.Git
	Usage       components.Usage
	Outdated    int
	Uses        []string
	UsedBy      []string
}

type requireInfo struct {
	path    string
	version string

	// indirect reports a requirement the module does not import itself, and
	// carries only to pin what a dependency of a dependency resolves to. It is
	// the "// indirect" comment go mod tidy writes.
	indirect bool
}

// versionRefs maps module path → dependency path → version.
type versionRefs map[string]map[string]string

// latestTags maps module path → latest tag.
type latestTags map[string]string
