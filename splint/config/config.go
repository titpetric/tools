// Package config reads .splint.yml, the file a tree states its house rules in.
//
// A tree with no such file takes the defaults.
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
	"slices"

	"github.com/goccy/go-yaml"

	"github.com/titpetric/tools/splint/importfmt"
	"github.com/titpetric/tools/splint/linters/filecheck"
	"github.com/titpetric/tools/splint/linters/imports"
	"github.com/titpetric/tools/splint/pathmatch"
)

// Filename is the file a tree states its rules in, read from the root of the
// tree under parse.
const Filename = ".splint.yml"

// Config is the file. Its accessors convert to the types the linters take,
// since a configuration type describes a document a person writes, and a
// linter takes data.
type Config struct {
	// Imports is the house rule for an import block.
	Imports Imports `yaml:"imports"`

	// Filecheck is the house rule for how long a file may run to.
	Filecheck Filecheck `yaml:"filecheck"`

	// Integration is the house rule for tests crossing a process boundary.
	Integration Integration `yaml:"integration"`
}

// Integration is the house rule for tests crossing a process boundary: they
// sit behind a build tag, in files named for it, so a plain go test runs the
// unit tests alone.
type Integration struct {
	// Enabled turns the linter on for the tree. A run naming the linter
	// with --linters turns it on as well.
	Enabled bool `yaml:"enabled"`

	// Tag is the build tag the integration tests sit behind. It defaults to
	// integration.
	Tag string `yaml:"tag,omitempty"`

	// Storage are the import paths whose constructors open the real storage.
	// An empty list reads any path ending in /storage or /store as one.
	Storage []string `yaml:"storage,omitempty"`

	// Depth is how many calls into the module a test's chain may run before
	// the call that crosses. It defaults to 3.
	Depth int `yaml:"depth,omitempty"`
}

// Imports is the house rule for an import block.
type Imports struct {
	// Order is the groups, in the order the file writes them.
	Order []string `yaml:"order,omitempty"`

	// Company are the path prefixes that form the company group.
	Company []string `yaml:"company,omitempty"`

	// Project is the module path that forms the project group. It defaults to
	// the module path of the tree under read.
	Project string `yaml:"project,omitempty"`

	// SetAlias writes an alias for a path ending in a major version. It
	// defaults to true.
	SetAlias *bool `yaml:"set-alias,omitempty"`

	// Aliases name what the tree cannot: an import path for a name no package
	// of the tree carries and no file writes yet.
	Aliases map[string]string `yaml:"aliases,omitempty"`

	// Pollution says when the linter reports a dependency reaching many files.
	Pollution Pollution `yaml:"pollution"`
}

// Pollution says when the linter reports a dependency reaching many files.
//
// The linter never reports a package of the tree, whatever its reach. A
// package structure exists so that every consumer under a model package
// imports it. The rule is about the dependencies a tree took on.
type Pollution struct {
	// PerPackage is how many files of one package may import one external
	// dependency before the linter reports it. Zero turns the rule off.
	PerPackage int `yaml:"per-package"`

	// FileShare is the share of the files of the tree one external dependency
	// may reach before the linter reports it, between 0 and 1. Zero turns the rule
	// off.
	FileShare float64 `yaml:"file-share"`

	// IncludeTests counts the test files. It defaults to true, so a package
	// whose three test files all reach testify shows up in the report. Set it
	// false when that reads as noise and not as a finding.
	IncludeTests *bool `yaml:"include-tests"`

	// Ignore are the dependencies the rule never reports, as import path
	// prefixes: an entry covers the path it names and every path below it.
	// It defaults to the golang.org/x modules and github.com/spf13/pflag. A
	// list in the file replaces the default, and an empty list clears it so
	// the rule checks every dependency.
	Ignore []string `yaml:"ignore"`
}

// Filecheck is the house rule for how long a file may run to.
//
// A tree stating nothing here gets the linter's own ceiling, so the
// section exists for the tree that has a file no number can name: a recursive
// descent parser, an interpreter loop, a test file the pairing rule will not
// let it split. The section names those by path and the linter still reports
// the rest of the tree.
type Filecheck struct {
	// MaxLines is how many lines of code a file may run to before the linter
	// reports it, blanks and comments already taken out. It defaults to 400.
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
	// path.Match cannot read fails the load of the file.
	Path string `yaml:"path"`

	// MaxLines is the ceiling for the files it covers. Zero means the rule
	// does not apply to them, the way a zero turns off a pollution rule.
	//
	// The first entry covering a file applies, so the linter reads a list top
	// to bottom and an author writes a narrow entry above the broad entry it
	// carves a hole in.
	MaxLines int `yaml:"max-lines"`
}

// Default is the configuration of a tree with no file.
func Default() *Config {
	yes := true
	return &Config{
		Imports: Imports{
			Order:     append([]string(nil), importfmt.DefaultOrder...),
			SetAlias:  &yes,
			Pollution: Pollution{PerPackage: 2, FileShare: 0.5, IncludeTests: &yes, Ignore: slices.Clone(imports.DefaultIgnore)},
		},
		Filecheck: Filecheck{MaxLines: filecheck.DefaultMaxLines},
	}
}

// Path is where the file sits for a tree rooted at dir.
func Path(dir string) string {
	return filepath.Join(dir, Filename)
}

// Load reads the file for a tree rooted at dir. A tree with no file gets the
// defaults, and no error.
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

// FilecheckAllowances is the file length rule as the linter takes it.
func (c *Config) FilecheckAllowances() filecheck.Allowances {
	out := make(filecheck.Allowances, 0, len(c.Filecheck.Allow))
	for _, entry := range c.Filecheck.Allow {
		out = append(out, filecheck.Allowance{Path: entry.Path, MaxLines: entry.MaxLines})
	}
	return out
}

// ImportOptions is the import rule as importfmt takes it, module heading the project group.
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
