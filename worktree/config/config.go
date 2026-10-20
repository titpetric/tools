// Package config holds the worktree configuration document and the setup
// screen that edits it.
//
// The document lives at FileName below the home directory. Load does not merge it with
// the built-in defaults: when the file exists it holds the whole configuration,
// and a setting it does not name reads as its zero value. Each setting name
// makes off the zero value, and Save writes every key
// back, so a round trip through the setup screen cannot drop one.
package config

import (
	"fmt"
	"slices"
)

// Version is the document version this build writes. Loading rejects a
// document declaring a higher version and accepts a document declaring none
// as unversioned.
const Version = 1

// Config is the worktree configuration document.
type Config struct {
	// Version is the document version. Zero means the document predates
	// versioning, or someone wrote it by hand without one.
	Version int `yaml:"version"`

	// Scan holds the workspace scan settings.
	Scan Scan `yaml:"scan"`
}

// Scan holds the settings of the workspace walk that collects git
// repositories and go modules.
type Scan struct {
	// EnableGitignore honours .gitignore files while walking. A checkout
	// below an ignored directory is not descended into, so it does not
	// appear at all; workspaces that gather git checkouts a parent
	// repository gitignores want this off.
	EnableGitignore bool `yaml:"enable_gitignore"`

	// EnableGitRepos lists git repositories that are not also go modules.
	EnableGitRepos bool `yaml:"enable_git_repos"`

	// IgnorePaths are directory names never descended into, whether a
	// .gitignore mentions them or not.
	IgnorePaths []string `yaml:"ignore_paths"`

	// RootMarkers are the files that mark the workspace root. The nearest
	// parent directory holding one of them is the scan root. With no
	// markers the scan starts in the current directory.
	RootMarkers []string `yaml:"root_markers"`
}

// Ignored reports whether IgnorePaths lists a directory name.
func (s Scan) Ignored(name string) bool {
	return slices.Contains(s.IgnorePaths, name)
}

// Default returns the built-in configuration, the values that apply when no
// document exists.
func Default() *Config {
	cfg, err := Parse(DefaultConfig)
	if err != nil {
		// The embedded document is part of the build, so a parse failure is
		// a programming error, and no run can recover from it.
		panic("config: parse embedded config.yml: " + err.Error())
	}
	return cfg
}

// Parse decodes a configuration document. Absent settings keep their zero
// value; Parse fills nothing in from the defaults.
func Parse(data []byte) (*Config, error) {
	cfg := &Config{}
	if err := decode(cfg, data); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Version > Version {
		return nil, fmt.Errorf("config version %d is newer than this build understands (%d)", cfg.Version, Version)
	}
	return cfg, nil
}
