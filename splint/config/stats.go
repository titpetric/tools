package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

// StatsFilename is the file the counters live in, under the directory the
// operating system keeps a user's configuration in.
//
// They are not kept beside the tree. A tree's own file states the rules its
// source is formatted under, which is a decision its authors made and commit;
// a counter is what one person's runs have done and belongs to that person's
// machine. Keeping it in the tree meant every run that rewrote a file left the
// repository dirty and every branch carried a different number.
const StatsFilename = "splint.yml"

// Stats is what runs of splint have counted.
type Stats struct {
	Imports StatsImports `yaml:"imports"`
}

// StatsImports is what the import fixer has counted.
type StatsImports struct {
	// Fixed is how many times a .go file has had its import block rewritten,
	// across every run this user has made, against any tree.
	Fixed int `yaml:"fixed"`
}

// StatsPath is the file the counters are kept in.
func StatsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, StatsFilename), nil
}

// LoadStats reads the counters. A machine that has none reads zeroes and no
// error.
func LoadStats() (Stats, error) {
	var stats Stats

	path, err := StatsPath()
	if err != nil {
		return stats, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return stats, nil
	}
	if err != nil {
		return stats, err
	}

	if err := yaml.Unmarshal(data, &stats); err != nil {
		return stats, err
	}
	return stats, nil
}

// AddFixed adds n to imports.fixed and writes the file back.
//
// The file is edited rather than re-encoded from the struct: somebody who
// wrote a comment beside a counter keeps it, and a key this version does not
// know stays where it was. The document is read whole with its comments kept
// aside, one value is changed, and the comments are written back where they
// were.
//
// A machine with no file gets one holding the count and nothing else. Adding
// nothing writes nothing.
func AddFixed(n int) error {
	if n <= 0 {
		return nil
	}

	path, err := StatsPath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		data = nil
	} else if err != nil {
		return err
	}

	document := yaml.MapSlice{}
	comments := yaml.CommentMap{}
	if len(data) > 0 {
		if err := yaml.UnmarshalWithOptions(data, &document, yaml.UseOrderedMap(), yaml.CommentToMap(comments)); err != nil {
			return err
		}
	}

	out, err := yaml.MarshalWithOptions(bump(document, n), yaml.WithComment(comments), yaml.Indent(2))
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return write(path, out)
}

// bump adds n to imports.fixed, keeping every other key as it was read. An
// imports that is not a mapping is replaced with one, which is what the value
// had to be for the counter to live under it.
func bump(document yaml.MapSlice, n int) yaml.MapSlice {
	for i, item := range document {
		if key, ok := item.Key.(string); !ok || key != "imports" {
			continue
		}
		imports, ok := item.Value.(yaml.MapSlice)
		if !ok {
			imports = yaml.MapSlice{}
		}
		document[i].Value = bumpFixed(imports, n)
		return document
	}

	return append(document, yaml.MapItem{Key: "imports", Value: bumpFixed(yaml.MapSlice{}, n)})
}

// bumpFixed adds n to the fixed entry, starting a counter that is missing or
// unreadable at zero.
func bumpFixed(imports yaml.MapSlice, n int) yaml.MapSlice {
	for i, item := range imports {
		if key, ok := item.Key.(string); !ok || key != "fixed" {
			continue
		}
		imports[i].Value = asInt(item.Value) + n
		return imports
	}

	return append(imports, yaml.MapItem{Key: "fixed", Value: n})
}

// asInt reads a decoded scalar as a count, and reads anything that is not one
// as zero.
func asInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case uint64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

// write replaces the file, through a temporary beside it so a run interrupted
// halfway leaves the file it found rather than half of one.
func write(path string, data []byte) error {
	temp := path + ".tmp"

	if err := os.WriteFile(temp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		os.Remove(temp)
		return err
	}

	return nil
}
