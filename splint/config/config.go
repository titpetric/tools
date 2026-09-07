// Package config reads .splint.yml, the file a tree states its house rules in.
//
// A tree with no such file is formatted by the defaults, which are the group
// order six of the eight goimports-reviser call sites in this workspace pass.
// The file exists to say the two things the defaults cannot know: which path
// prefixes the project calls its own, and what a name means when the tree
// itself does not say.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/titpetric/tools/splint/importfmt"
)

// Filename is the file a tree states its rules in, read from the root of the
// tree being parsed.
const Filename = ".splint.yml"

// Config is the file.
type Config struct {
	// Imports is the house rule for an import block.
	Imports Imports `yaml:"imports"`
}

// Imports is the house rule for an import block.
type Imports struct {
	// Order is the groups, in the order they are written.
	Order []string `yaml:"order,omitempty"`

	// Company are the path prefixes that form the company group.
	Company []string `yaml:"company,omitempty"`

	// Project is the module path that forms the project group. It defaults to
	// the module path of the tree being read.
	Project string `yaml:"project,omitempty"`

	// SetAlias writes an alias for a path ending in a major version. It
	// defaults to true.
	SetAlias *bool `yaml:"set-alias,omitempty"`

	// Aliases name what the tree cannot: an import path for a name no package
	// of the tree is called and no file writes yet.
	Aliases map[string]string `yaml:"aliases,omitempty"`

	// Pollution is when a dependency reaching many files is reported.
	Pollution Pollution `yaml:"pollution"`
}

// Pollution is when a dependency reaching many files is reported.
//
// A package of the tree is never reported however far it reaches. A model
// package imported by every consumer under it is what a package structure is
// for. The rule is about the dependencies a tree took on.
type Pollution struct {
	// PerPackage is how many files of one package may import one external
	// dependency before it is reported. Zero turns the rule off.
	PerPackage int `yaml:"per-package"`

	// FileShare is the share of the files of the tree one external dependency
	// may reach before it is reported, between 0 and 1. Zero turns the rule
	// off.
	FileShare float64 `yaml:"file-share"`

	// IncludeTests counts the test files. It defaults to true, so a package
	// whose three test files all reach testify is reported. Set it false when
	// that reads as noise rather than as a finding.
	IncludeTests *bool `yaml:"include-tests"`
}

// Default is the configuration a tree with no file is read under.
func Default() *Config {
	yes := true
	return &Config{
		Imports: Imports{
			Order:     append([]string(nil), importfmt.DefaultOrder...),
			SetAlias:  &yes,
			Pollution: Pollution{PerPackage: 2, FileShare: 0.5, IncludeTests: &yes},
		},
	}
}

// Path is where the file sits for a tree rooted at dir.
func Path(dir string) string {
	return filepath.Join(dir, Filename)
}

// Load reads the file for a tree rooted at dir. A tree with no file is read
// under the defaults, and no error.
func Load(dir string) (*Config, error) {
	cfg := Default()

	path := Path(dir)

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	if len(cfg.Imports.Order) == 0 {
		cfg.Imports.Order = append([]string(nil), importfmt.DefaultOrder...)
	}

	return cfg, nil
}

// ImportOptions is the import rule as importfmt takes it. The module path is
// the one the tree builds under, and is what the project group defaults to.
func (c *Config) ImportOptions(module string) importfmt.Options {
	project := c.Imports.Project
	if project == "" {
		project = module
	}

	return importfmt.Options{
		Order:    c.Imports.Order,
		Company:  c.Imports.Company,
		Project:  project,
		SetAlias: c.Imports.SetAlias == nil || *c.Imports.SetAlias,
	}
}

// CountTests reports whether the pollution rule counts the test files.
func (p Pollution) CountTests() bool {
	return p.IncludeTests == nil || *p.IncludeTests
}
