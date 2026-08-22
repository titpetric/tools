// Package config holds the worktree configuration document and the setup
// screen that edits it.
//
// The document lives at ~/.config/worktree.yml. It is not merged with the
// built-in defaults: when the file exists it is the whole configuration, and
// a setting it does not name reads as its zero value. Every setting is
// therefore named so that off is the zero value, and Save writes every key
// back, so a round trip through the setup screen cannot drop one.
package config

import "slices"

// Version is the document version this build writes. A document declaring a
// higher version is rejected, a document declaring none is accepted as
// unversioned.
const Version = 1

// Config is the worktree configuration document.
type Config struct {
	// Version is the document version. Zero means the document predates
	// versioning, or was hand written without one.
	Version int `yaml:"version"`

	// Scan holds the workspace scan settings.
	Scan Scan `yaml:"scan"`
}

// Scan holds the settings of the workspace walk that collects git
// repositories and go modules.
type Scan struct {
	// EnableGitignore honours .gitignore files while walking. A checkout
	// below an ignored directory is not descended into, so it does not
	// appear at all; workspaces that consolidate git checkouts a parent
	// repository gitignores want this off.
	EnableGitignore bool `yaml:"enable_gitignore"`

	// EnableGitRepos lists git repositories that are not also go modules.
	EnableGitRepos bool `yaml:"enable_git_repos"`

	// IgnorePaths are directory names never descended into, whether or not
	// a .gitignore mentions them.
	IgnorePaths []string `yaml:"ignore_paths"`

	// RootMarkers are the files that mark the workspace root. The nearest
	// parent directory holding one of them is the scan root. With no
	// markers the current directory is used.
	RootMarkers []string `yaml:"root_markers"`
}

// Ignored reports whether a directory name is listed in IgnorePaths.
func (s Scan) Ignored(name string) bool {
	return slices.Contains(s.IgnorePaths, name)
}
