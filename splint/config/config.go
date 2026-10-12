// Package config reads .splint.yml, the file a tree states its house rules in.
//
// A tree with no such file is formatted by the defaults.
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

	"github.com/goccy/go-yaml"

	"github.com/titpetric/tools/splint/importfmt"
	"github.com/titpetric/tools/splint/linters/filecheck"
	"github.com/titpetric/tools/splint/pathmatch"
)

// Filename is the file a tree states its rules in, read from the root of the
// tree being parsed.
const Filename = ".splint.yml"

// Config is the file.
type Config struct {
	// Imports is the house rule for an import block.
	Imports Imports `yaml:"imports"`

	// Filecheck is the house rule for how long a file may run to.
	Filecheck Filecheck `yaml:"filecheck"`
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

// Filecheck is the house rule for how long a file may run to.
//
// A tree stating nothing here is read under the linter's own ceiling, so the
// section exists for the tree that has a file no number can name: a recursive
// descent parser, an interpreter loop, a test file the pairing rule will not
// let it split. Those are named by path and the rest of the tree is left
// reported.
type Filecheck struct {
	// MaxLines is how many lines of code a file may run to before it is
	// reported, blanks and comments already taken out. It defaults to 400.
	// Zero turns the rule off for every file the allow list does not name.
	MaxLines int `yaml:"max-lines"`

	// Allow are the paths that run under a ceiling of their own. A file no
	// entry covers runs under MaxLines.
	Allow []FileAllowance `yaml:"allow,omitempty"`
}

// FileAllowance is one ceiling and the paths it applies to.
type FileAllowance struct {
	// Path is the pattern, matched against the file as a report names it: the
	// package directory and the filename, with no leading dot or slash. The
	// syntax is path.Match per segment, so a star stops at a separator, plus
	// ** for any number of segments. parser/*.go is the files of one
	// directory and **/*_test.go is every test file of the tree. A pattern
	// path.Match cannot read is refused when the file is loaded.
	Path string `yaml:"path"`

	// MaxLines is the ceiling for the files it covers. Zero means the rule
	// does not apply to them, the way a zero turns off a pollution rule.
	//
	// The first entry covering a file is the one that applies, so a list is
	// read top to bottom and a narrow entry is written above the broad entry
	// it carves a hole in.
	MaxLines int `yaml:"max-lines"`
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
		Filecheck: Filecheck{MaxLines: filecheck.DefaultMaxLines},
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

	if err := cfg.Filecheck.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return cfg, nil
}

// validate refuses a rule no reader can have meant. A ceiling is a count of
// lines, a pattern is a pattern, and an entry naming no path would quietly
// cover nothing for the life of the file.
func (f Filecheck) validate() error {
	if f.MaxLines < 0 {
		return fmt.Errorf("filecheck.max-lines is %d, which is not a number of lines", f.MaxLines)
	}

	for index, entry := range f.Allow {
		if entry.Path == "" {
			return fmt.Errorf("filecheck.allow[%d] names no path", index)
		}
		if err := pathmatch.Valid(entry.Path); err != nil {
			return fmt.Errorf("filecheck.allow[%d]: %q is not a path pattern: %w", index, entry.Path, err)
		}
		if entry.MaxLines < 0 {
			return fmt.Errorf("filecheck.allow[%d]: max-lines is %d, which is not a number of lines", index, entry.MaxLines)
		}
	}

	return nil
}

// FilecheckAllowances is the file length rule as the linter takes it. The two
// types carry the same two fields and are declared apart on purpose: config
// describes a document a person writes, and a linter is handed data.
func (c *Config) FilecheckAllowances() filecheck.Allowances {
	out := make(filecheck.Allowances, 0, len(c.Filecheck.Allow))
	for _, entry := range c.Filecheck.Allow {
		out = append(out, filecheck.Allowance{Path: entry.Path, MaxLines: entry.MaxLines})
	}
	return out
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
